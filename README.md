# siok: Consul Service Health Aggregator API

*Picture this*: you have a need to know that some network-attached service is functioning correctly, but the service in question doesn't include a ```/health``` route, it doesn't cache its health-check results and/or the health-checks don't cover all the dependent services **and** you're running Consul while you're reading this; well you're in luck: this API is for *you*.

## What is siok

**siok** is a simple API which aggregates and translates the health of a service a Consul Agent registers and monitors, alongside the health of the related entities, such as node health, service- and node-maintenance, in a JSON array of all entities related to the service with a HTTP 200 status code if all are "passing" and/or "warning", 503 otherwise.

In other words, you register a service **bar** with one health-check on the Consul-enabled host **foo**, you send a GET request to ```http://foo:31998/health?service=bar```, you get this HTTP 200 response back:

```json
[
    {
        "CheckID": "service:foo",
        "Name": "Service 'foo' check",
        "Notes": "",
        "Output": "Everything is OK!\n",
        "ServiceID": "foo",
        "Status": "passing"
    }
]
```

## A typical scenario for siok

To expand the example above, let's say that you have a service running on host **foo**, the name of the service is **bar**. **bar** doesn't have a ```/health```, ```/status``` or a similar route, so to get its health you must query it as its client would, parse the results and see if they are okay. You create a Consul health-check for this service, on its local Consul Agent, to do this and you run it every 10s, because hey, this query is *expensive*. Then, you also want to verify does some service **baz**, on which **bar** depends on, is also okay. You attach this check to **bar** as well, but you schedule it to run every 5s, because **baz** is a simpler service and the requests to it are cheaper. Then, you also want to know does a simple TCP connect work on **bar**, and you schedule this one to run every 1s, because it's hella cheap.

You end up with the service **bar** having three health-checks, but your HAProxy can (easily) only query one HTTP endpoint to get the health? The solution would be to run **siok** and point the HAProxy to

```
http://foo:31998/health?service=bar
```

to get the health-check details regarding the service **bar** on host **foo**.

## Building siok

1. Clone the repo and cd to its directory
2. If you have Golang already installed, just run ```make```

    Or, if you have Docker installed and you want to build **siok** using the Golang Docker image:

    ```bash
    docker run --rm -v "$PWD":/usr/src/siok -w /usr/src/siok golang:1.8 make
    ```

## Running siok

**siok** supports only one flag, ```-p```, to specify the port it'll run on. The default is **31998**.

It expects the Consul Agent HTTP API on ```http://localhost:8500```.

It only has one route, ```/health``` and only one query string parameter at the moment, ```service```, to specify the Consul ```ServiceID``` on the node in question. Note that ```ServiceID``` *can* differ from the ```ServiceName```, so make sure you get it right. Ie. this is how an URL should look like:

```
/health?service=$serviceID
```

Expect HTTP 200 if "passing"/"warning", 503 otherwise. On top of that, additional info is in the JSON array returned and a ```Warning``` HTTP response header is included if any of the checks are in the "warning" state.

## Details

**siok** uses the responses from the local Consul Agent and doesn't query the Consul Catalog at all. That's why it needs to run local to the Consul Agent which monitors the services it has registered and that makes it really lightweight and fast, so my guess is that it'll scale nicely. Most of the requests finish in under one millisecond: for example, when testing it with 2000 remote requests with the concurrency of 100, with one Consul Service underneath, I got *all* responses back within 250 milliseconds (yeah, *all* 2000 requests), so I guess you could say it's fast.

## TL;DR

Use Consul Agent to register the service and its health-checks. Use **siok** to see the service's health remotely.
