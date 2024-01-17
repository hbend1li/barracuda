# reaction

A daemon that scans program outputs for repeated patterns, and takes action.

A common usage is to scan ssh and webserver logs, and to ban hosts that cause multiple authentication errors.

🚧 This program hasn't received external audit. however, it already works well on my servers 🚧

## Rationale

I was using the honorable fail2ban since quite a long time, but i was a bit frustrated by its cpu consumption
and all its heavy default configuration.

In my view, a security-oriented program should be simple to configure
and an always-running daemon should be implemented in a fast*er* language.

reaction does not have all the features of the honorable fail2ban, but it's ~10x faster and has more manageable configuration.

[📽️ quick french name explanation 😉](https://u.ppom.me/reaction.webm)

[🇬🇧 in-depth blog article](https://blog.ppom.me/en-reaction)
/ [🇫🇷 french version](https://blog.ppom.me/fr-reaction)

## Configuration

YAML and [JSONnet](https://jsonnet.org/) (more powerful) are supported.
both are extensions of JSON, so JSON is transitively supported.

- See [reaction.yml](./app/example.yml) or [reaction.jsonnet](./config/example.jsonnet) for a fully explained reference
- See [server.jsonnet](./config/server.jsonnet) for a real-world configuration
- See [reaction.example.service](./config/reaction.example.service) for a systemd service file
- This quick example shows what's needed to prevent brute force attacks on an ssh server:

<details open>

<summary><code>/etc/reaction.yml</code></summary>

```yaml
patterns:
  ip: '(([ 0-9 ]{1,3}\.){3}[0-9]{1,3})|([0-9a-fA-F:]{2,90})'

start:
  - [ 'ip46tables', '-w', '-N', 'reaction' ]
  - [ 'ip46tables', '-w', '-A', 'reaction', '-j', 'ACCEPT' ]
  - [ 'ip46tables', '-w', '-I', 'reaction', '1', '-s', '127.0.0.1', '-j', 'ACCEPT' ]
  - [ 'ip46tables', '-w', '-I', 'INPUT', '-p', 'all', '-j', 'reaction' ]

stop:
  - [ 'ip46tables', '-w', '-D', 'INPUT', '-p', 'all', '-j', 'reaction' ]
  - [ 'ip46tables', '-w', '-F', 'reaction' ]
  - [ 'ip46tables', '-w', '-X', 'reaction' ]

streams:
  ssh:
    cmd: [ 'journalctl', '-fu', 'sshd.service' ]
    filters:
      failedlogin:
        regex:
          - 'authentication failure;.*rhost=<ip>'
        retry: 3
        retryperiod: '6h'
        actions:
          ban:
            cmd: [ 'ip46tables', '-w', '-I', 'reaction', '1', '-s', '<ip>', '-j', 'block' ]
          unban:
            cmd: [ 'ip46tables', '-w', '-D', 'reaction', '1', '-s', '<ip>', '-j', 'block' ]
            after: '48h'
```

</details>

<details>

<summary><code>/etc/reaction.jsonnet</code></summary>

```jsonnet
local iptables(args) = [ 'ip46tables', '-w' ] + args;
local banFor(time) = {
  ban: {
    cmd: iptables(['-A', 'reaction', '-s', '<ip>', '-j', 'reaction-log-refuse']),
  },
  unban: {
    after: time,
    cmd: iptables(['-D', 'reaction', '-s', '<ip>', '-j', 'reaction-log-refuse']),
  },
};
{
  patterns: {
    ip: {
      regex: @'(?:(?:[ 0-9 ]{1,3}\.){3}[0-9]{1,3})|(?:[0-9a-fA-F:]{2,90})',
    },
  },
  start: [
    iptables([ '-N', 'reaction' ]),
    iptables([ '-A', 'reaction', '-j', 'ACCEPT' ]),
    iptables([ '-I', 'reaction', '1', '-s', '127.0.0.1', '-j', 'ACCEPT' ]),
    iptables([ '-I', 'INPUT', '-p', 'all', '-j', 'reaction' ]),
  ],
  stop: [
    iptables([ '-D', 'INPUT', '-p', 'all', '-j', 'reaction' ]),
    iptables([ '-F', 'reaction' ]),
    iptables([ '-X', 'reaction' ]),
  ],
  streams: {
    ssh: {
      cmd: [ 'journalctl', '-fu', 'sshd.service' ],
      filters: {
        failedlogin: {
          regex: [ @'authentication failure;.*rhost=<ip>' ],
          retry: 3,
          retryperiod: '6h',
          actions: banFor('48h'),
        },
      },
    },
  },
}
```

</details>


### Database

The embedded database is stored in the working directory.
If you don't know where to start reaction, `/var/lib/reaction` should be a sane choice.

### CLI

- `reaction start` runs the server
- `reaction show` show pending actions (ie. bans)
- `reaction flush` permits to run pending actions (ie. clear bans)
- `reaction test-regex` permits to test regexes
- `reaction help` for full usage.

### `ip46tables`

`ip46tables` is a minimal c program present in its own subdirectory with only standard posix dependencies.

It permits to configure `iptables` and `ip6tables` at the same time.
It will execute `iptables` when detecting ipv4, `ip6tables` when detecting ipv6 and both if no ip address is present on the command line.

## Wiki

You'll find more ressources, service configurations, etc. on the [Wiki](https://reaction.ppom.me)!

## Installation

[![Packaging status](https://repology.org/badge/vertical-allrepos/reaction-fail2ban.svg)](https://repology.org/project/reaction-fail2ban/versions)

### Binaries

Executables are provided [here](https://framagit.org/ppom/reaction/-/releases/), for a standard x86-64 linux machine.

A standard place to put such executables is `/usr/local/bin/`.

> Provided binaries in the previous section are compiled this way:
```shell
$ docker run -it --rm -e HOME=/tmp/ -v $(pwd):/tmp/code -w /tmp/code -u $(id -u) golang:1.20 make clean reaction.deb
$ make signaturese
```
#### Signature verification

Starting at v1.0.3, all binaries are signed with public key `RWSpLTPfbvllNqRrXUgZzM7mFjLUA7PQioAItz80ag8uU4A2wtoT2DzX`. You can check their authenticity with minisign:
```bash
minisign -VP RWSpLTPfbvllNqRrXUgZzM7mFjLUA7PQioAItz80ag8uU4A2wtoT2DzX -m ip46tables
minisign -VP RWSpLTPfbvllNqRrXUgZzM7mFjLUA7PQioAItz80ag8uU4A2wtoT2DzX -m reaction
# or
minisign -VP RWSpLTPfbvllNqRrXUgZzM7mFjLUA7PQioAItz80ag8uU4A2wtoT2DzX -m reaction.deb
```

#### Debian

The releases also contain a `reaction.deb` file, which packages reaction & ip46tables.
You can install it using `sudo apt install ./reaction.deb`.
You'll have to create a configuration at `/etc/reaction.jsonnet`.

If you want to use another configuration format (YAML or JSON), you can override systemd's `ExecStart` command in `/etc/systemd/system/reaction.service` like this:
```systemd
[Service]
# First an empty directive to reset the default one
ExecStart=
# Then put what you want
ExecStart=/usr/bin/reaction start -c /etc/reaction.yml
```

#### NixOS

- [ package ](https://framagit.org/ppom/nixos/-/blob/main/pkgs/reaction/default.nix)
- [ module ](https://framagit.org/ppom/nixos/-/blob/main/modules/common/reaction.nix)

### Compilation

You'll need the go (>= 1.20) toolchain for reaction and a c compiler for ip46tables.
```shell
$ make
```
Don't hesitate to take a look at the `Makefile` to understand what's happening!

### Installation

To install the binaries
```shell
make install
```

To install the systemd file as well
```shell
make install_systemd
```
