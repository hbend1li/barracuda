# Serveur

`reactiond <FILEPATH>`

Avec un défaut à `/etc/reaction/reactiond.conf`

```yaml
definitions:
  - &iptablesban iptables -I reaction 1 -s <ip> -j block
  - &iptablesunban iptables -D reaction 1 -s <ip> -j block

regexes:
  ip: '(([0-9]{1,3}\.){3}[0-9]{1,3})|([0-9a-fA-F:]{2,90})'

streams:
  nextcloud:
    cmd: journalctl -fu phpfpm-nextcloud.service
    filters:
      failed-login:
        regex:
          - '"message":"Login failed: .\+ (Remote IP: <ip>)"'
        retry: 3
        retry-period: 1h
        actions:
          ban:
            cmd: *iptablesban
          unban:
            cmd: *iptablesunban 
            after: 1h
```

reactionc: le client
