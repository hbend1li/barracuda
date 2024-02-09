{
  patterns: {
    num: {
      regex: '[0-9]+',
      ignore: ['1'],
      ignoreregex: ['2.?'],
    },
  },

  streams: {
    tailDown1: {
      cmd: ['sh', '-c', "echo 1 2 3 4 5 11 12 21 22 33 | tr ' ' '\n' | while read i; do sleep 1; echo found $i; done"],
      filters: {
        findIP: {
          regex: ['^found <num>$'],
          retry: 1,
          retryperiod: '30s',
          actions: {
            damn: {
              cmd: ['echo', '<num>'],
            },
            undamn: {
              cmd: ['echo', 'undamn', '<num>'],
              after: '4s',
              onexit: true,
            },
          },
        },
      },
    },
  },
}
