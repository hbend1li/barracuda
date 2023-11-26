// This is the extensive configuration used on a **real** server!

local banFor(time) = {
  ban: {
    cmd: ['ip46tables', '-w', '-A', 'reaction', '-s', '<ip>', '-j', 'reaction-log-refuse'],
  },
  unban: {
    after: time,
    cmd: ['ip46tables', '-w', '-D', 'reaction', '-s', '<ip>', '-j', 'reaction-log-refuse'],
  },
};

{
  patterns: {
    // IPs can be IPv4 or IPv6
    // ip46tables (C program also in this repo) handles running the good commands
    ip: {
      regex: @'(([0-9]{1,3}\.){3}[0-9]{1,3})|([0-9a-fA-F:]{2,90})',
    },
  },

  start: [
    ['ip46tables', '-w', '-N', 'reaction'],
    ['ip46tables', '-w', '-I', 'INPUT', '-p', 'all', '-j', 'reaction'],
  ],
  stop: [
    ['ip46tables', '-w', '-D', 'INPUT', '-p', 'all', '-j', 'reaction'],
    ['ip46tables', '-w', '-F', 'reaction'],
    ['ip46tables', '-w', '-X', 'reaction'],
  ],

  streams: {
    // Ban hosts failing to connect via ssh
    ssh: {
      cmd: ['journalctl', '-fn0', '-u', 'sshd.service'],
      filters: {
        failedlogin: {
          regex: [
            @'authentication failure;.*rhost=<ip>',
            @'Connection reset by authenticating user .* <ip>',
            @'Failed password for .* from <ip>',
          ],
          retry: 3,
          retryperiod: '6h',
          actions: banFor('48h'),
        },
      },
    },

    // Ban hosts which knock on closed ports.
    // It needs this iptables chain to be used to drop packets:
    // ip46tables -N log-refuse
    // ip46tables -A log-refuse -p tcp --syn -j LOG --log-level info --log-prefix 'refused connection: '
    // ip46tables -A log-refuse -m pkttype ! --pkt-type unicast -j nixos-fw-refuse
    // ip46tables -A log-refuse -j DROP
    kernel: {
      cmd: ['journalctl', '-fn0', '-k'],
      filters: {
        portscan: {
          regex: ['refused connection: .*SRC=<ip>'],
          retry: 4,
          retryperiod: '1h',
          actions: banFor('720h'),
        },
      },
    },
    // Note: nextcloud and vaultwarden could also be filters on the nginx stream
    // I did use their own logs instead because it's less logs to parse than the front webserver

    // Ban hosts failing to connect to Nextcloud
    nextcloud: {
      cmd: ['journalctl', '-fn0', '-u', 'phpfpm-nextcloud.service'],
      filters: {
        failedLogin: {
          regex: [
            @'"remoteAddr":"<ip>".*"message":"Login failed:',
            @'"remoteAddr":"<ip>".*"message":"Trusted domain error.',
          ],
          retry: 3,
          retryperiod: '1h',
          actions: banFor('1h'),
        },
      },
    },

    // Ban hosts failing to connect to vaultwarden
    vaultwarden: {
      cmd: ['journalctl', '-fn0', '-u', 'vaultwarden.service'],
      filters: {
        failedlogin: {
          actions: banFor('2h'),
          regex: [@'Username or password is incorrect\. Try again\. IP: <ip>\. Username:'],
          retry: 3,
          retryperiod: '1h',
        },
      },
    },

    // Used with this nginx log configuration:
    // log_format withhost '$remote_addr - $remote_user [$time_local] $host "$request" $status $bytes_sent "$http_referer" "$http_user_agent"';
    // access_log /var/log/nginx/access.log withhost;
    nginx: {
      cmd: ['tail', '-n0', '-f', '/var/log/nginx/access.log'],
      filters: {
        // Ban hosts failing to connect to Directus
        directus: {
          regex: [
            @'^<ip> .* "POST /auth/login HTTP/..." 401 [0-9]+ .https://directusdomain',
          ],
          retry: 6,
          retryperiod: '4h',
          actions: banFor('4h'),
        },

        // Ban hosts presenting themselves as bots of ChatGPT
        gptbot: {
          regex: [@'^<ip>.*GPTBot/1.0'],
          action: banFor('720h'),
        },

        // Ban hosts failing to connect to slskd
        slskd: {
          regex: [
            @'^<ip> .* "POST /api/v0/session HTTP/..." 401 [0-9]+ .https://slskddomain',
          ],
          retry: 3,
          retryperiod: '1h',
          actions: banFor('6h'),
        },

        // Ban suspect HTTP requests
        // Those are frequent malicious requests I got from bots
        // Make sure you don't have honnest use cases for those requests, or your clients may be banned for 2 weeks!
        suspectRequests: {
          regex: [
            // (?:[^/" ]*/)* is a "non-capturing group" regex that allow for subpath(s)
            // example: /code/.env should be matched as well as /.env
            //           ^^^^^
            @'^<ip>.*"GET /(?:[^/" ]*/)*\.env ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*info\.php ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*owa/auth/logon.aspx ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*auth.html ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*auth1.html ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*password.txt ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*passwords.txt ',
            @'^<ip>.*"GET /(?:[^/" ]*/)*dns-query ',
            // Do not include this if you have a Wordpress website ;)
            @'^<ip>.*"GET /(?:[^/" ]*/)*wp-login\.php',
            @'^<ip>.*"GET /(?:[^/" ]*/)*wp-includes',
            // Do not include this if a client must retrieve a config.json file ;)
            @'^<ip>.*"GET /(?:[^/" ]*/)*config\.json ',
          ],
          action: banFor('720h'),
        },
      },
    },
  },
}
