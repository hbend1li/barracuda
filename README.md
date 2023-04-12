# reaction

🚧 this program has not been tested in production yet 🚧

a program that scans program outputs, such as logs,
for repeated patterns, such as failed login attempts,
and takes action, such as banning ips.

(adapted from [fail2ban](http://fail2ban.org)'s presentation 😄)

## rationale

i was using fail2ban since quite a long time, but i was a bit frustrated by it's cpu consumption
and all its heavy default configuration.

in my view, a security-oriented program should be simple to configure (`sudo` is a very bad exemple!)
and an always-running daemon should be implemented in a fast language.

## configuration

this configuration file is all that should be needed to prevent bruteforce attacks on an ssh server.

`/etc/reaction.yml`
```yaml
definitions:
  - &iptablesban [ "iptables" "-w" "-I" "reaction" "1" "-s" "<ip>" "-j" "block" ]
  - &iptablesunban [ "iptables" "-w" "-D" "reaction" "1" "-s" "<ip>" "-j" "block" ]

patterns:
  ip: '(([0-9]{1,3}\.){3}[0-9]{1,3})|([0-9a-fA-F:]{2,90})'

streams:
  ssh:
    cmd: [ "journalctl" "-fu" "sshd.service" ]
    filters:
      failedlogin:
        regex:
          - authentication failure;.*rhost=<ip>
        retry: 3
        retry-period: 6h
        actions:
          ban:
            cmd: *iptablesban
          unban:
            cmd:  *iptablesunban
            after: 2d
```

`/etc/systemd/system/reaction.service`
```systemd
[Unit]
WantedBy=multi-user.target

[Service]
ExecStart=/path/to/reaction -c /etc/reaction.yml

ExecStartPre=/path/to/iptables -w -N reaction
ExecStartPre=/path/to/iptables -w -A reaction -j ACCEPT
ExecStartPre=/path/to/iptables -w -I INPUT -p all -j reaction

ExecStopPost=/path/to/iptables -w -D INPUT -p all -j reaction
ExecStopPost=/path/to/iptables -w -F reaction
ExecStopPost=/path/to/iptables -w -X reaction

StateDirectory=reaction
WorkingDirectory=/var/lib/reaction
```
See [reaction.service](./reaction.service) and [reaction.yml](./reaction.yml) for the fully commented examples.

### database

the working directory of `reaction` will be used to create and read from the embedded [lmdb](https://www.symas.com/lmdb) database.
if you don't know where to start it, `/var/lib/reaction` should be a sane choice.

### compilation

```shell
$ go build .
```
