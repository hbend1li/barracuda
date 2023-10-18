// Those strings will be substitued in each shell() call
local substitutions = [
  ['OUTFILE', '"$HOME/.local/share/watch/logs-$(date +%F)"'],
  ['DATE', '"$(date "+%F %T")"'],
];

// Substitue each substitutions' item in string
local sub(str) = std.foldl(
  (function(changedstr, kv) std.strReplace(changedstr, kv[0], kv[1])),
  substitutions,
  str
);
local shell(prg) = [
  'sh',
  '-c',
  sub(prg),
];

local log(line) = shell('echo DATE ' + std.strReplace(line, '\n', ' ') + '>> OUTFILE');

{
  start: [
    shell('mkdir -p "$(dirname OUTFILE)"'),
    log('start'),
  ],

  stop: [
    log('stop'),
  ],

  patterns: {
    all: { regex: '.*' },
  },

  streams: {
    // Be notified about each window focus change
    // FIXME DOESN'T WORK
    sway: {
      cmd: shell(|||
        swaymsg -rm -t subscribe "['window']" | jq -r 'select(.change == "focus") | .container | if has("app_id") and .app_id != null then .app_id else .window_properties.class end'
      |||),
      filters: {
        send: {
          regex: ['^<all>$'],
          actions: {
            send: { cmd: log('focus <all>') },
          },
        },
      },
    },

    // Be notified when user is away
    swayidle: {
      // FIXME echo stop and start instead?
      cmd: ['swayidle', 'timeout', '30', 'echo sleep', 'resume', 'echo resume'],
      filters: {
        send: {
          regex: ['^<all>$'],
          actions: {
            send: { cmd: log('<all>') },
          },
        },
      },
    },

    // Be notified about tmux activity
    // Limitation: can't handle multiple concurrently attached sessions
    // tmux: {
    //   cmd: shell(|||
    //     LAST_TIME="0"
    //     LAST_ACTIVITY=""
    //     while true;
    //     do
    //       NEW_TIME=$(tmux display -p '#{session_activity}')
    //       if [ -n "$NEW_TIME" ] && [ "$NEW_TIME" -gt "$LAST_TIME" ]
    //       then
    //         LAST_TIME="$NEW_TIME"
    //         NEW_ACTIVITY="$(tmux display -p '#{pane_current_command} #{pane_current_path}')"
    //         if [ -n "$NEW_ACTIVITY" ] && [ "$NEW_ACTIVITY" != "$LAST_ACTIVITY" ]
    //         then
    //           LAST_ACTIVITY="$NEW_ACTIVITY"
    //           echo "tmux $NEW_ACTIVITY"
    //         fi
    //       fi
    //       sleep 10
    //     done
    //   |||),
    //   filters: {
    //     send: {
    //       regex: ['^tmux <all>$'],
    //       actions: {
    //         send: { cmd: log('tmux <all>') },
    //       },
    //     },
    //   },
    // },

    // Be notified about firefox activity
    // TODO
  },
}
