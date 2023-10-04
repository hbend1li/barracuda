# reaction

a program that scans program outputs, such as logs,
for repeated patterns, such as failed login attempts,
and takes action, such as banning ips.

(adapted from [fail2ban](http://fail2ban.org)'s presentation 😄)

🚧 this program hasn't received external audit. however, it already works well on my servers 🚧

## rationale

i was using fail2ban since quite a long time, but i was a bit frustrated by its cpu consumption
and all its heavy default configuration.

in my view, a security-oriented program should be simple to configure (`sudo` is a very bad example!)
and an always-running daemon should be implemented in a fast language.

<a href="https://u.ppom.me/reaction.webm">📽️ french example</a>

## configuration

this configuration file is all that should be needed to prevent brute force attacks on an ssh server.

see [reaction.service](./config/reaction.service) and [reaction.yml](./app/reaction.yml) for the fully explained examples.

`/etc/reaction.yml`
```yaml
definitions:
  - &iptablesban [ "iptables" "-w" "-I" "reaction" "1" "-s" "<ip>" "-j" "block" ]
  - &iptablesunban [ "iptables" "-w" "-D" "reaction" "1" "-s" "<ip>" "-j" "block" ]

patterns:
  ip: '(([0-9]{1,3}\.){3}[0-9]{1,3})|([0-9a-fA-F:]{2,90})'
  ignore:
    - '127.0.0.1'
    - '::1'

streams:
  ssh:
    cmd: [ "journalctl" "-fu" "sshd.service" ]
    filters:
      failedlogin:
        regex:
          - 'authentication failure;.*rhost=<ip>'
        retry: 3
        retryperiod: '6h'
        actions:
          ban:
            cmd: *iptablesban
          unban:
            cmd: *iptablesunban
            after: '48h'
```

jsonnet is also supported:

`/etc/reaction.jsonnet`
```jsonnet
local iptablesban = ['iptables', '-w', '-A', 'reaction', '1', '-s', '<ip>', '-j', 'DROP'];
local iptablesunban = ['iptables', '-w', '-D', 'reaction', '1', '-s', '<ip>', '-j', 'DROP'];
{
  patterns: {
    ip: {
      regex: @'(?:(?:[0-9]{1,3}\.){3}[0-9]{1,3})|(?:[0-9a-fA-F:]{2,90})',
      ignore: ['127.0.0.1', '::1'],
    },
  },
  streams: {
    ssh: {
      cmd: ['journalctl', '-fu', 'sshd.service'],
      filters: {
        failedlogin: {
          regex: [ @'authentication failure;.*rhost=<ip>' ],
          retry: 3,
          retryperiod: '6h',
          actions: {
            ban: {
              cmd: iptablesban,
            },
            unban: {
              cmd: iptablesunban,
              after: '48h',
              onexit: true,
            },
          },
        },
      },
    },
  },
}
```

note that both yaml and jsonnet are extensions of json, so it is also inherently supported.

`/etc/systemd/system/reaction.service`
```systemd
[Unit]
WantedBy=multi-user.target

[Service]
ExecStart=/path/to/reaction -c /etc/reaction.yml

ExecStartPre=/path/to/iptables -w -N reaction
ExecStartPre=/path/to/iptables -w -A reaction -j ACCEPT
ExecStartPre=/path/to/iptables -w -I reaction 1 -s 127.0.0.1 -j ACCEPT
ExecStartPre=/path/to/iptables -w -I INPUT -p all -j reaction

ExecStopPost=/path/to/iptables -w -D INPUT -p all -j reaction
ExecStopPost=/path/to/iptables -w -F reaction
ExecStopPost=/path/to/iptables -w -X reaction

StateDirectory=reaction
RuntimeDirectory=reaction
WorkingDirectory=/var/lib/reaction
```

### database

the working directory of `reaction` will be used to create and read from the embedded database.
if you don't know where to start it, `/var/lib/reaction` should be a sane choice.

### socket

the socket allowing communication between the cli and server will be created at `/run/reaction/reaction.socket`.

### compilation

you'll need the go toolchain.
```shell
$ go build .
```

### nixos

in addition to the [package](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/pkgs/reaction/default.nix)
and [module](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/common/reaction.nix)
that i didn't try to upstream to nixpkgs yet (although they are ready), i use extensively reaction on my servers. if you're using nixos,
consider reading and building upon [my own building blocks](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/common/reaction-variables.nix),
[my own non-root reaction conf, including conf for SSH, port scanning & Nginx common attack URLS](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/common/reaction-custom.nix),
and the configuration for [nextcloud](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/musi/file.ppom.me.nix#L53),
[vaultwarden](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/musi/vaultwarden.nix#L45),
and [maddy](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/musi/mail.nix#L74). see also an [example](https://framagit.org/ppom/nixos/-/blob/cf5448b21ae3386265485308a6cd077e8068ad77/modules/musi/mail.nix#L85) where it does something else than banning IPs.
