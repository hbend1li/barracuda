{
  patterns: {
    num: {
      regex: '[0-9]+',
    },
  },

  streams: {
    tailDown1: {
      cmd: ['sh', '-c', "echo 01 02 03 04 05 | tr ' ' '\n' | while read i; do sleep 0.5; echo found $i; done"],
      filters: {
        findIP1: {
          regex: ['^found <num>$'],
          retry: 1,
          retryperiod: '2m',
          actions: {
            damn: {
              cmd: ['echo', '<num>'],
            },
            undamn: {
              cmd: ['echo', 'undamn', '<num>'],
              after: '1m',
              onexit: true,
            },
          },
        },
      },
    },
    tailDown2: {
      cmd: ['sh', '-c', "echo 11 12 13 14 15 11 13 15 | tr ' ' '\n' | while read i; do sleep 0.3; echo found $i; done"],
      filters: {
        findIP2: {
          regex: ['^found <num>$'],
          retry: 2,
          retryperiod: '2m',
          actions: {
            damn: {
              cmd: ['echo', '<num>'],
            },
            undamn: {
              cmd: ['echo', 'undamn', '<num>'],
              after: '1m',
              onexit: true,
            },
          },
        },
      },
    },
  },
}
