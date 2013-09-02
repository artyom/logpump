# logpump: submit syslog messages to Scribe

Logpump follows syslog files and pushes messages to [Scribe][].

	Usage: logpump -host <host> -port <port> -conffile <config.json>
	  -conffile="": configuration file
	  -host="localhost": scribe host
	  -port=1463: scribe port

Configuration file example:

	[
		{
			"Pattern":"/var/log/messages*",
			"Statefile":"/var/cache/logpump/messages_state.json"
		},
		{
			"Pattern":"/var/log/daemon.log*",
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

## Features

* State persistence: logpump can be safely stopped and started again, it would
  read only previously unread log messages.
* Logfiles rotation recognized, even between logpump runs.
* gzip/bzip2 compressed files support.
* Automatic reconnect to Scribe server: try hard to reconnect in case of
  network problems.
* Automatic message resubmit if Scribe answered "TRY LATER".
* Session statistics: total messages sent, connection retries, message
  resubmits.

## Bugs


[Scribe]: https://github.com/facebook/scribe
