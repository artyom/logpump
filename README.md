# logpump: submit syslog messages to Scribe

Logpump follows syslog files and pushes messages to [Scribe][].

	Usage: logpump -host <host> -port <port> -conffile <config.(json|yaml)>
	  -conffile="": configuration file (json or yaml)
	  -host="localhost": scribe host
	  -nohostnameprefix=false: do not set hostname as a default prefix
	  -port=1463: scribe port
	  -reconnectforever=false: try to reconnect forever instead of default 10 retries
	  -statedir="": directory to save state files if none was given in config


Configuration file example (json):

	[
		{
			"Pattern":"/var/log/messages*",
			"Statefile":"/var/cache/logpump/messages_state.json"
		},
		{
			"Pattern":"/var/log/daemon.log*",
			"CheckRotate": "1m30s",
			"Statefile":"/var/cache/logpump/daemon_state.json",
			"Category": "important",
			"Prefix":"DAEMONLOG"
		}
	]

This configuration tells logpump to watch for messages in files
`/var/log/messages*` and `/var/log/daemon.log*` and push them to Scribe server,
saving state to files `/var/cache/logpump/messages_state.json` and
`/var/cache/logpump/daemon_state.json` respectively. Messages from `daemon.log`
would have prefix "`DAEMONLOG: `" appended to them, and category `important`
assigned.

If no new messages seen in `daemon.log` for 1.5 minutes, matching files are
re-checked for rotation. This delay defaults to 2 minutes, min. allowed value
20s.

The same configuration in yaml format:

	-
	  pattern: /var/log/messages*
	  statefile: /var/cache/logpump/messages_state.json

	-
	  pattern: /var/log/daemon.log*
	  checkrotate: 1m30s
	  statefile: /var/cache/logpump/daemon_state.json
	  category: important
	  prefix: DAEMONLOG

## Features

* syslogd / svlogd naming formats support (see svlogd support notes below).
* State persistence: logpump can be safely stopped and started again, it would
  read only previously unread log messages.
* Logfiles rotation recognized, even between logpump runs.
* gzip/bzip2 compressed files support.
* Automatic reconnect to Scribe server: try hard to reconnect in case of
  network problems.
* Automatic message resubmit if Scribe answered "TRY LATER".
* Session statistics: total messages sent, connection retries, message
  resubmits.

## Usage Examples

	logpump -host 192.168.0.1 -conffile /etc/logpump.json -reconnectforever

This would start logpump in "reconnect forever" mode: it would indefinitely try
to (re)connect to remote server. Use with care, this mode can mask potential
network misconfigurations.

	logpump -host 192.168.0.1 -conffile /etc/logpump.json -nohostnameprefix

By default, each line sent is prefixed with host name unless prefix is
overridden in `config.json`. Option `-nohostnameprefix` disables this behavior,
so no prefix is added to log lines unless set in configuration file.

	logpump -host 192.168.0.1 -conffile /etc/logpump.json -statedir /var/cache/logpump

If no Statefile option specified for any Pattern in configuration file,
statefile name would be automatically generated from pattern; the file would be
loaded/saved to/from directory given with `-statedir` option.

`logpump` writes diagnostic messages to stderr, as well as periodic statistics
on total messages sent, reconnects and retries (Scribe server asked to try
later).

## svlogd support

If you use svlogd to write logs (like `svlogd /path/to/dir`), you can also use logpump to pipe them into scribe. Just give it `/path/to/dir/*` as patten and it would handle files properly.

## Setup

Via `go get`:

	go get -u -v github.com/artyom/logpump

If any dependencies are broken, use vendored copies:

	cd $GOPATH/src/github.com/artyom/logpump
	GOPATH=$PWD/_vendor:$GOPATH go install -v

On Debian/Ubuntu you can build deb package with `dpkg-buildpackage`.

[Scribe]: https://github.com/facebook/scribe
