# A website
This is my personal website, feel free to steal the server and look at the code if there's anything useful there.
There's not much to it, but it currently:
- uses `blackfriday`/`github_flavored_markdown` to convert markdown into HTML
- has a `.service` file that lets it run automatically on startup
- daemonizes itself using `go-daemon` so you get nice log and pid files
- has a set of `iptables-persistent` rules to avoid needing to run as root

TODOs
- make a cronjob to automatically renew the certificate
- add a header bar and make the footer look a little nicer
- do cool stuff so I can post about it here I guess
