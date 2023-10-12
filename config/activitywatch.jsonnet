local directory = '~/.local/share/watch';
// Those strings will be substitued in each shell() call
local substitutions = [
  ['OUTFILE', directory + '/logs-$(date %+F)'],
  ['TMUXFILE', directory + '/tmux'],
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

{
  // Startup is currently not implemented
  startup: shell(|||
    mkdir -p "$(dirname OUTFILE)"
    echo DATE start >> OUTFILE
    # tmux set-hook -g pane-focus-in[50] new-session -d 'echo tmux >> TMUXFILE'
  |||),

  // Stop is currently not implemented
  stop: shell(|||
    tmux set-hook -ug pane-focus-in[50]
    echo DATE stop >> OUTFILE
  |||),

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
            send: { cmd: shell('echo DATE focus <all> >> OUTFILE') },
          },
        },
      },
    },

    // Be notified when user is away
    swayidle: {
      cmd: ['swayidle', 'timeout', '60', 'echo sleep', 'resume', 'echo resume'],
      filters: {
        send: {
          regex: ['^<all>$'],
          actions: {
            send: { cmd: shell('echo DATE <all> >> OUTFILE') },
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
    //         send: { cmd: shell('echo DATE tmux <all> >> OUTFILE') },
    //       },
    //     },
    //   },
    // },

    // Be notified about firefox activity
    // TODO
  },
}
