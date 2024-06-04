// package: gotestapi.v1alpha
// file: gotestapi/v1alpha/gotestapi.proto

var gotestapi_v1alpha_gotestapi_pb = require("../../gotestapi/v1alpha/gotestapi_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var VeggieShop = (function () {
  function VeggieShop() {}
  VeggieShop.serviceName = "gotestapi.v1alpha.VeggieShop";
  return VeggieShop;
}());

VeggieShop.CreatePurchase = {
  methodName: "CreatePurchase",
  service: VeggieShop,
  requestStream: false,
  responseStream: false,
  requestType: gotestapi_v1alpha_gotestapi_pb.CreatePurchaseRequest,
  responseType: gotestapi_v1alpha_gotestapi_pb.CreatePurchaseResponse
};

exports.VeggieShop = VeggieShop;

function VeggieShopClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

VeggieShopClient.prototype.createPurchase = function createPurchase(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(VeggieShop.CreatePurchase, {
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

exports.VeggieShopClient = VeggieShopClient;

