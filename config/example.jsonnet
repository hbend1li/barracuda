// This file is using JSONNET, a complete configuration language based on JSON
// See https://jsonnet.org
// JSONNET is a superset of JSON, so one can write plain JSON files if wanted.
// Note that YAML is also supported.

// variables defined for later use.
local iptablesban = ['ip46tables', '-w', '-A', 'reaction', '1', '-s', '<ip>', '-j', 'DROP'];
local iptablesunban = ['ip46tables', '-w', '-D', 'reaction', '1', '-s', '<ip>', '-j', 'DROP'];
// ip46tables is a minimal C program (only POSIX dependencies) present as a subdirectory.
// it permits to handle both ipv4/iptables and ipv6/ip6tables commands

{
  // patterns are substitued in regexes.
  // when a filter performs an action, it replaces the found pattern
  patterns: {
    ip: {
      // reaction regex syntax is defined here: https://github.com/google/re2/wiki/Syntax
      // jsonnet's @'string' is for verbatim strings
      regex: @'(?:(?:[0-9]{1,3}\.){3}[0-9]{1,3})|(?:[0-9a-fA-F:]{2,90})',
      ignore: ['127.0.0.1', '::1'],
    },
  },

  // streams are commands
  // they're run and their ouptut is captured
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
            'authentication failure;.*rhost=<ip>',
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
              // JSONNET substitutes the variable (defined at the beginning of the file)
              cmd: iptablesban,
            },
            unban: {
              cmd: iptablesunban,
              // if after is defined, the action will not take place immediately, but after a specified duration
              // same format as retryperiod
              after: '48h',
              // let's say reaction is quitting. does it run all those pending commands which had an `after` duration set?
              // if you want reaction to run those pending commands before exiting, you can set this:
              onexit: true,
              // (defaults to false)
              // here it is not useful because we will flush the chain containing the bans anyway
              // (see /conf/reaction.service)
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
