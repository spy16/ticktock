# Gorilla v2

In this model, we launch 2 goroutines per connection for reading and writing.

- Reader uses blocking IO implemented by the Go stdlib.
- The broker uses control and message channel combined with a single management goroutine to safely access the subscription map.

## Benchmark Results

Run using: `k6 run loadtest.js -d 30s -u 2000`

```
checks................: 100.00% ✓ 12000         ✗ 0
data_received.........: 49 MB   1.6 MB/s
data_sent.............: 4.1 MB  131 kB/s
iteration_duration....: avg=5.13s   min=5s     med=5.13s   max=5.43s    p(90)=5.27s   p(95)=5.3s
iterations............: 12000   386.993322/s
vus...................: 488     min=488         max=2000
vus_max...............: 2000    min=2000        max=2000
ws_connecting.........: avg=35.53ms min=79.2µs med=32.41ms max=241.34ms p(90)=70.96ms p(95)=113.45ms
ws_msgs_received......: 4670162 150610.125609/s
ws_msgs_sent..........: 50935   1642.625405/s
ws_session_duration...: avg=5.13s   min=5s     med=5.12s   max=5.43s    p(90)=5.27s   p(95)=5.3s
ws_sessions...........: 12000   386.993322/s
```