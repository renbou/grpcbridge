service "testsvc1" {
  target = "127.0.0.1:50052"
}

service "testsvc2" {
  target = "scheme://testsvc2:grpc"
}

service "testsvc3" {
  target = "https://127.0.0.1:50054"
}
