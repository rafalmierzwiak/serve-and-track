# serve-and-track

A tracking web server.

Main function is to serve an image and log requests in apache log format. Served are also service status (health established based on presence of a state file), and service metrics (via use of Prometheus client library).

* server

```
$ go run serve_and_track.go
2017/08/14 14:47:50 INFO http: Server started :8080
::1 - - [14/Aug/2017:14:47:57 +0100] "GET /state HTTP/1.1" 200 2 "" "curl/7.54.0"
::1 - - [14/Aug/2017:14:48:06 +0100] "GET /state HTTP/1.1" 503 33 "" "curl/7.54.0"
::1 - - [14/Aug/2017:14:48:14 +0100] "GET /track HTTP/1.1" 200 42 "" "curl/7.54.0"
::1 - - [14/Aug/2017:14:48:25 +0100] "GET /metrics HTTP/1.1" 200 5150 "" "curl/7.54.0"
::1 - - [14/Aug/2017:14:48:36 +0100] "GET /track HTTP/1.1" 200 42 "" "curl/7.54.0"
::1 - - [14/Aug/2017:14:48:41 +0100] "GET /metrics HTTP/1.1" 200 5150 "" "curl/7.54.0"
2017/08/14 14:49:11 INFO http: Server stopping
2017/08/14 14:49:11 INFO http: Server stopped gracefully
```

* client

```
$ touch ./state && curl -sS --write-out "http_code: %{http_code}, size_download: %{size_download}" --output /dev/null http://localhost:8080/state
http_code: 200, size_download: 2

$ rm -f ./state && curl -sS --write-out "http_code: %{http_code}, size_download: %{size_download}" --output /dev/null http://localhost:8080/state
http_code: 503, size_download: 33

$ curl -sS --write-out "http_code: %{http_code}, size_download: %{size_download}" --output /dev/null http://localhost:8080/track
http_code: 200, size_download: 42

$ curl -sS http://localhost:8080/metrics | grep "^tracking_"
tracking_request_duration{quantile="0.5"} 1.9946e-05
tracking_request_duration{quantile="0.9"} 1.9946e-05
tracking_request_duration{quantile="0.99"} 1.9946e-05
tracking_request_duration_sum 1.9946e-05
tracking_request_duration_count 1
tracking_requests_count_total{status="success"} 1
tracking_requests_size_total 42

$ curl -sS http://localhost:8080/track | file -b --mime-type -
image/gif

$ curl -sS http://localhost:8080/metrics | grep "^tracking_"
tracking_request_duration{quantile="0.5"} 9.125e-06
tracking_request_duration{quantile="0.9"} 1.9946e-05
tracking_request_duration{quantile="0.99"} 1.9946e-05
tracking_request_duration_sum 2.9071e-05
tracking_request_duration_count 2
tracking_requests_count_total{status="success"} 2
tracking_requests_size_total 84
```
