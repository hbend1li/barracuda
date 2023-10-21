{
  patterns: {
    num: {
      regex: '[0-9]+',
    },
  },

  streams: {
    tailDown1: {
      cmd: ['sh', '-c', "echo 1 2 3 4 5 1 2 3 4 5 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 | tr ' ' '\n' | while read i; do sleep 2; echo found $(($i % 10)); done"],
      filters: {
        findIP: {
          regex: ['^found <num>$'],
          retry: 3,
          retryperiod: '30s',
          actions: {
            damn: {
              cmd: ['echo', '<num>'],
            },
            undamn: {
              cmd: ['echo', 'undamn', '<num>'],
              after: '30s',
              onexit: true,
            },
          },
        },
      },
    },
    tailDown2: {
      cmd: ['sh', '-c', 'echo coucou; sleep 2m'],
      // cmd: ['sh', '-c', "echo 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 6 7 8 9 | tr ' ' '\n' | while read i; do sleep 3; echo found $(($i % 60)); done"],
      filters: {
        findIP: {
          regex: ['^found <num>$'],
          retry: 3,
          retryperiod: '30s',
          actions: {
            damn: {
              cmd: ['echo', '<num>'],
            },
            undamn: {
              cmd: ['echo', 'undamn', '<num>'],
              after: '30s',
              onexit: true,
            },
          },
        },
      },
    },
  },
}
