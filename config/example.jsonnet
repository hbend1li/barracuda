// This file is using JSONNET, a complete configuration language based on JSON
// See https://jsonnet.org
// JSONNET is a superset of JSON, so one can write plain JSON files if wanted.
// Note that YAML is also supported, see ./example.yml

// JSONNET functions
local iptables(args) = ['ip46tables', '-w'] + args;
// ip46tables is a minimal C program (only POSIX dependencies) present in a subdirectory of this repo.
// it permits to handle both ipv4/iptables and ipv6/ip6tables commands

{
  // patterns are substitued in regexes.
  // when a filter performs an action, it replaces the found pattern
  patterns: {
    ip: {
      // reaction regex syntax is defined here: https://github.com/google/re2/wiki/Syntax
      // jsonnet's @'string' is for verbatim strings
      // simple version: regex: @'(?:(?:[0-9]{1,3}\.){3}[0-9]{1,3})|(?:[0-9a-fA-F:]{2,90})',
      regex: @'(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}|(?:(?:[0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,7}:|(?:[0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,5}(?::[0-9a-fA-F]{1,4}){1,2}|(?:[0-9a-fA-F]{1,4}:){1,4}(?::[0-9a-fA-F]{1,4}){1,3}|(?:[0-9a-fA-F]{1,4}:){1,3}(?::[0-9a-fA-F]{1,4}){1,4}|(?:[0-9a-fA-F]{1,4}:){1,2}(?::[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:(?:(?::[0-9a-fA-F]{1,4}){1,6})|:(?:(?::[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(?::[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(?:ffff(?::0{1,4}){0,1}:){0,1}(?:(?:25[0-5]|(?:2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(?:25[0-5]|(?:2[0-4]|1{0,1}[0-9]){0,1}[0-9])|(?:[0-9a-fA-F]{1,4}:){1,4}:(?:(?:25[0-5]|(?:2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(?:25[0-5]|(?:2[0-4]|1{0,1}[0-9]){0,1}[0-9]))',
      ignore: ['127.0.0.1', '::1'],
    },
  },

  // Those commands will be executed in order at start, before everything else
  start: [
    // Create an iptables chain for reaction
    iptables(['-N', 'reaction']),
    // Insert this chain as the first item of the INPUT chain (for incoming connections)
    iptables(['-I', 'INPUT', '-p', 'all', '-j', 'reaction']),
  ],

  // Those commands will be executed in order at stop, after everything else
  stop: [
    // Remove the chain from the INPUT chain
    iptables(['-D', 'INPUT', '-p', 'all', '-j', 'reaction']),
    // Empty the chain
    iptables(['-F', 'reaction']),
    // Delete the chain
    iptables(['-X', 'reaction']),
  ],

  // streams are commands
  // they are run and their ouptut is captured
  // *example:* `tail -f /var/log/nginx/access.log`
  // their output will be used by one or more filters
  streams: {
    // streams have a user-defined name
    ssh: {
      // note that if the command is not in environment's `PATH`
      // its full path must be given.
      cmd: ['journalctl', '-n0', '-fu', 'sshd.service'],
      // filters run actions when they match regexes on a stream
      filters: {
        // filters have a user-defined name
        failedlogin: {
          // reaction's regex syntax is defined here: https://github.com/google/re2/wiki/Syntax
          regex: [
            // <ip> is predefined in the patterns section
            // ip's regex is inserted in the following regex
            @'authentication failure;.*rhost=<ip>',
            @'Failed password for .* from <ip>',
            @'Connection reset by authenticating user .* <ip>',
          ],
          // if retry and retryperiod are defined,
          // the actions will only take place if a same pattern is
          // found `retry` times in a `retryperiod` interval
          retry: 3,
          // format is defined here: https://pkg.go.dev/time#ParseDuration
          retryperiod: '6h',
          // actions are run by the filter when regexes are matched
          actions: {
            // actions have a user-defined name
            ban: {
              cmd: iptables(['-A', 'reaction', '-s', '<ip>', '-j', 'DROP']),
            },
            unban: {
              cmd: iptables(['-D', 'reaction', '-s', '<ip>', '-j', 'DROP']),
              // if after is defined, the action will not take place immediately, but after a specified duration
              // same format as retryperiod
              after: '48h',
              // let's say reaction is quitting. does it run all those pending commands which had an `after` duration set?
              // if you want reaction to run those pending commands before exiting, you can set this:
              // onexit: true,
              // (defaults to false)
              // here it is not useful because we will flush and delete the chain containing the bans anyway
              // (with the stop commands)
            },
          },
        },
      },
    },
  },
}

// persistence

// tldr; when an `after` action is set in a filter, such filter acts as a 'jail',
// which is persisted after reboots.

// full;
// when a filter is triggered, there are 2 flows:
//
// if none of its actions have an `after` directive set:
// no action will be replayed.
//
// else (if at least one action has an `after` directive set):
// if reaction stops while `after` actions are pending:
// and reaction starts again while those actions would still be pending:
// reaction executes the past actions (actions without after or with then+after < now)
// and plans the execution of future actions (actions with then+after > now)
