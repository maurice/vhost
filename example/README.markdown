Example
=======

See `example_config.json` for the available config options.

Testing
-------

You might be able to use `backend.go` as a simple server to 
provide canned responses, to test your `vhost` config, eg:

	$ go run backend.go -port 8081 -saying "Howdy ho!"

	$ curl localhost:8081
	Howdy ho
