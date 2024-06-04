// package: gotestapi.v1
// file: gotestapi/v1/gotestapi.proto

var gotestapi_v1_gotestapi_pb = require("./gotestapi_pb.cjs");
var grpc = require("@improbable-eng/grpc-web").grpc;

var VeggieShopService = (function () {
  function VeggieShopService() {}
  VeggieShopService.serviceName = "gotestapi.v1.VeggieShopService";
  return VeggieShopService;
})();

VeggieShopService.CreateShop = {
  methodName: "CreateShop",
  service: VeggieShopService,
  requestStream: false,
  responseStream: false,
  requestType: gotestapi_v1_gotestapi_pb.CreateShopRequest,
  responseType: gotestapi_v1_gotestapi_pb.CreateShopResponse,
};

VeggieShopService.GetShopStats = {
  methodName: "GetShopStats",
  service: VeggieShopService,
  requestStream: false,
  responseStream: false,
  requestType: gotestapi_v1_gotestapi_pb.GetStatsRequest,
  responseType: gotestapi_v1_gotestapi_pb.GetStatsResponse,
};

VeggieShopService.BuildPurchase = {
  methodName: "BuildPurchase",
  service: VeggieShopService,
  requestStream: true,
  responseStream: false,
  requestType: gotestapi_v1_gotestapi_pb.BuildPurchaseRequest,
  responseType: gotestapi_v1_gotestapi_pb.BuildPurchaseResponse,
};

VeggieShopService.MonitorShop = {
  methodName: "MonitorShop",
  service: VeggieShopService,
  requestStream: false,
  responseStream: true,
  requestType: gotestapi_v1_gotestapi_pb.MonitorShopRequest,
  responseType: gotestapi_v1_gotestapi_pb.MonitorShopResponse,
};

exports.VeggieShopService = VeggieShopService;

function VeggieShopServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

VeggieShopServiceClient.prototype.createShop = function createShop(
  requestMessage,
  metadata,
  callback
) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(VeggieShopService.CreateShop, {
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
    },
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    },
  };
};

VeggieShopServiceClient.prototype.getShopStats = function getShopStats(
  requestMessage,
  metadata,
  callback
) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(VeggieShopService.GetShopStats, {
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
    },
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    },
  };
};

VeggieShopServiceClient.prototype.buildPurchase = function buildPurchase(
  metadata
) {
  var listeners = {
    end: [],
    status: [],
  };
  var client = grpc.client(VeggieShopService.BuildPurchase, {
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
  });
  client.onEnd(function (status, statusMessage, trailers) {
    listeners.status.forEach(function (handler) {
      handler({ code: status, details: statusMessage, metadata: trailers });
    });
    listeners.end.forEach(function (handler) {
      handler({ code: status, details: statusMessage, metadata: trailers });
    });
    listeners = null;
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    write: function (requestMessage) {
      if (!client.started) {
        client.start(metadata);
      }
      client.send(requestMessage);
      return this;
    },
    end: function () {
      client.finishSend();
    },
    cancel: function () {
      listeners = null;
      client.close();
    },
  };
};

VeggieShopServiceClient.prototype.monitorShop = function monitorShop(
  requestMessage,
  metadata
) {
  var listeners = {
    data: [],
    end: [],
    status: [],
  };
  var client = grpc.invoke(VeggieShopService.MonitorShop, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners.end.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    },
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    },
  };
};

exports.VeggieShopServiceClient = VeggieShopServiceClient;
