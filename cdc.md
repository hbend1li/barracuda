# Serveur

`reactiond <FILEPATH>`

Avec un défaut à `/etc/reaction/reactiond.conf`

```yaml
actions:
  iptables:
    
regexes:
  IP: '(([0-9]{1,3}\.){3}[0-9]{1,3})|([0-9a-fA-F:]{2,90})'

streams:
 nextcloud:
   cmd: journalctl -fu phpfpm-nextcloud.service
   actions:
     - regex: '"message":"Login failed: .\+ (Remote IP: \(?<IP>[0-9a-fA-F.:]\+\))"'
       # Can also be a list
       cmd: iptables -I f2b-nextcloud 1 -s <ip> -j <blocktype>
```

reactionc: le client
