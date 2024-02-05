#include<ctype.h>
#include<errno.h>
#include<stdio.h>
#include<stdlib.h>
#include<string.h>
#include<unistd.h>

// If this programs
// - receives an ipv4 address in its arguments:
//   → it will executes iptables  with the same arguments in place.
//
// - receives an ipv6 address in its arguments:
//   → it will executes ip6tables with the same arguments in place.
//
// - doesn't receive an ipv4 or ipv6 address in its arguments:
//   → it will executes both, with the same arguments in place.

int isIPv4(char *tab) {
	int i,len;
	// IPv4 addresses are at least 7 chars long
	len = strlen(tab);
	if (len < 7 || !isdigit(tab[0]) || !isdigit(tab[len-1])) {
			return 0;
	}
	// Each char must be a digit or a dot between 2 digits
	for (i=1; i<len-1; i++) {
		if (!isdigit(tab[i]) && !(tab[i] == '.' && isdigit(tab[i-1]) && isdigit(tab[i+1]))) {
			return 0;
		}
	}
	return 1;
}

int isIPv6(char *tab) {
	int i,len, twodots = 0;
	// IPv6 addresses are at least 3 chars long
	len = strlen(tab);
	if (len < 3) {
			return 0;
	}
	// Each char must be a digit, :, a-f, or A-F
	for (i=0; i<len; i++) {
		if (!isdigit(tab[i]) && tab[i] != ':' && !(tab[i] >= 'a' && tab[i] <= 'f') && !(tab[i] >= 'A' && tab[i] <= 'F')) {
			return 0;
		}
	}
	return 1;
}

int guess_type(int len, char *tab[]) {
	int i;
	for (i=0; i<len; i++) {
		if (isIPv4(tab[i])) {
			return 4;
		} else if (isIPv6(tab[i])) {
			return 6;
		}
	}
	return 0;
}

int exec(char *str, char **argv) {
	argv[0] = str;
	execvp(str, argv);
	// returns only if fails
	printf("ip46tables: exec failed %d\n", errno);
}

int main(int argc, char **argv) {
	if (argc < 2) {
		printf("ip46tables: At least one argument has to be given\n");
		exit(1);
	}
	int type;
	type = guess_type(argc, argv);
	if (type == 4) {
		exec("iptables", argv);
	} else if (type == 6) {
		exec("ip6tables", argv);
	} else {
		pid_t pid = fork();
		if (pid == -1) {
			printf("ip46tables: fork failed\n");
			exit(1);
		} else if (pid) {
			exec("iptables", argv);
		} else {
			exec("ip6tables", argv);
		}
	}
}
