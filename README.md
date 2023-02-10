## cnames

Take a list of resolved domains and / or subdomains and output any associated CNAMES. Accepts input via stdin or as an argument

## Recommended Usage

`$ cat subdomains | filter-resolved | cnames`

or 

`$ assetfinder -subs-only example.com | filter-resolved | cnames -c 50 | sort -u`

or

`$ cnames sub.example.com`

or if you want to see which subdomain has the associated CNAME, use the verbose flag:

`$ cat subdomains | filter-resolved | cnames -v`


## Install

If you have Go installed and configured (i.e. with $GOPATH/bin in your $PATH):

`go install github.com/cybercdh/cnames@latest`
