#include<ctype.h>
#include<errno.h>
#include<stdio.h>
#include<stdlib.h>
#include<string.h>
#include<unistd.h>

// nft46 'add element inet reaction ipvXbans { 1.2.3.4 }' → nft 'add element inet reaction ipv4bans { 1.2.3.4 }'
// nft46 'add element inet reaction ipvXbans { a:b::c:d }' → nft 'add element inet reaction ipv6bans { a:b::c:d }'
//
// the character X is replaced by 4 or 6 depending on the address family of the specified IP
//
// Limitations:
// - nft46 must receive exactly one argument
// - only one IP must be given per command
// - the IP must be between { braces }

int isIPv4(char *tab, int len) {
	int i;
	// IPv4 addresses are at least 7 chars long
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

int isIPv6(char *tab, int len) {
	int i;
	// IPv6 addresses are at least 3 chars long
	if (len < 3) {
		return 0;
	}
	// Each char must be a digit, :, a-f, or A-F
	for (i=0; i<len; i++) {
		if (!isdigit(tab[i]) && tab[i] != ':' && tab[i] != '.' && !(tab[i] >= 'a' && tab[i] <= 'f') && !(tab[i] >= 'A' && tab[i] <= 'F')) {
			return 0;
		}
	}
	return 1;
}

int findchar(char *tab, char c, int i, int len) {
	while (i < len && tab[i] != c) i++;
	if (i == len) {
		printf("nft46: one %c must be present", c);
		exit(1);
	}
	return i;
}

void adapt_args(char *tab) {
	int i, len, X, startIP, endIP, startedIP;
	X = startIP = endIP = -1;
	startedIP = 0;
	len = strlen(tab);
	i = 0;
	X = i = findchar(tab, 'X', i, len);
	startIP = i = findchar(tab, '{', i, len);
	while (startIP + 1 <= (i = findchar(tab, ' ', i, len))) startIP = i + 1;
	i = startIP;
	endIP = i = findchar(tab, ' ', i, len) - 1;

	if (isIPv4(tab+startIP, endIP-startIP+1)) {
		tab[X] = '4';
		return;
	}

	if (isIPv6(tab+startIP, endIP-startIP+1)) {
		tab[X] = '6';
		return;
	}

	printf("nft46: no IP address found\n");
	exit(1);
}

int exec(char *str, char **argv) {
	argv[0] = str;
	execvp(str, argv);
	// returns only if fails
	printf("nft46: exec failed %d\n", errno);
}

int main(int argc, char **argv) {
	if (argc != 2) {
		printf("nft46: Exactly one argument must be given\n");
		exit(1);
	}
	adapt_args(argv[1]);
	exec("nft", argv);
}
