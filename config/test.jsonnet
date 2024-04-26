{
  patterns: {
    num: {
      regex: '[0-9]+',
      ignore: ['1'],
      // ignoreregex: ['2.?'],
    },
    letter: {
      regex: '[a-z]+',
      ignore: ['b'],
      // ignoreregex: ['b.?'],
    },
  },

  streams: {
    tailDown1: {
      cmd: ['sh', '-c', "echo 1_a 2_a 3_a a_1 a_2 a_3 | tr ' ' '\n' | while read i; do sleep 1; echo found $i; done"],
      filters: {
        findIP: {
          regex: [
            '^found <num>_<letter>$',
            '^found <letter>_<num>$',
          ],
          retry: 2,
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
