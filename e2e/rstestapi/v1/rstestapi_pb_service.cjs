// package: rstestapi.v1
// file: rstestapi/v1/rstestapi.proto

var rstestapi_v1_rstestapi_pb = require("./rstestapi_pb.cjs");
var google_protobuf_empty_pb = require("google-protobuf/google/protobuf/empty_pb");
var google_protobuf_timestamp_pb = require("google-protobuf/google/protobuf/timestamp_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ClockService = (function () {
  function ClockService() {}
  ClockService.serviceName = "rstestapi.v1.ClockService";
  return ClockService;
}());

ClockService.GetTime = {
  methodName: "GetTime",
  service: ClockService,
  requestStream: false,
  responseStream: false,
  requestType: google_protobuf_empty_pb.Empty,
  responseType: google_protobuf_timestamp_pb.Timestamp
};

exports.ClockService = ClockService;

function ClockServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ClockServiceClient.prototype.getTime = function getTime(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ClockService.GetTime, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

exports.ClockServiceClient = ClockServiceClient;

