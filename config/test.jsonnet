{
  patterns: {
    num: {
      regex: '[0-9]+',
    },
  },

  start: [
    ['err'],
    ['sleep', '1'],
  ],

  stop: [
    ['sleep', '1'],
    // ['false'],
    ['true'],
  ],

  streams: {
    tailDown1: {
      cmd: ['sh', '-c', "echo 1 2 3 4 5 | tr ' ' '\n' | while read i; do sleep 2; echo found $(($i % 10)); done"],
      // cmd: ['sh', '-c', "echo 1 2 3 4 5 1 2 3 4 5 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 1 2 3 4 | tr ' ' '\n' | while read i; do sleep 2; echo found $(($i % 10)); done"],
      filters: {
        findIP: {
          regex: ['^found [ <num>$'],
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
    // tailDown2: {
    //   cmd: ['sh', '-c', 'echo coucou; sleep 2m'],
    //   filters: {
    //     findIP: {
    //       regex: ['^found <num>$'],
    //       retry: 3,
    //       retryperiod: '30s',
    //       actions: {
    //         damn: {
    //           cmd: ['echo', '<num>'],
    //         },
    //         undamn: {
    //           cmd: ['echo', 'undamn', '<num>'],
    //           after: '30s',
    //           onexit: true,
    //         },
    //       },
    //     },
    //   },
    // },
  },
}
