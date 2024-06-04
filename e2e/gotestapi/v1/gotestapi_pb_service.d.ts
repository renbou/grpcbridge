// package: gotestapi.v1
// file: gotestapi/v1/gotestapi.proto

import * as gotestapi_v1_gotestapi_pb from "../../gotestapi/v1/gotestapi_pb";
import {grpc} from "@improbable-eng/grpc-web";

type VeggieShopServiceCreateShop = {
  readonly methodName: string;
  readonly service: typeof VeggieShopService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof gotestapi_v1_gotestapi_pb.CreateShopRequest;
  readonly responseType: typeof gotestapi_v1_gotestapi_pb.CreateShopResponse;
};

type VeggieShopServiceGetShopStats = {
  readonly methodName: string;
  readonly service: typeof VeggieShopService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof gotestapi_v1_gotestapi_pb.GetStatsRequest;
  readonly responseType: typeof gotestapi_v1_gotestapi_pb.GetStatsResponse;
};

type VeggieShopServiceBuildPurchase = {
  readonly methodName: string;
  readonly service: typeof VeggieShopService;
  readonly requestStream: true;
  readonly responseStream: false;
  readonly requestType: typeof gotestapi_v1_gotestapi_pb.BuildPurchaseRequest;
  readonly responseType: typeof gotestapi_v1_gotestapi_pb.BuildPurchaseResponse;
};

type VeggieShopServiceMonitorShop = {
  readonly methodName: string;
  readonly service: typeof VeggieShopService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof gotestapi_v1_gotestapi_pb.MonitorShopRequest;
  readonly responseType: typeof gotestapi_v1_gotestapi_pb.MonitorShopResponse;
};

export class VeggieShopService {
  static readonly serviceName: string;
  static readonly CreateShop: VeggieShopServiceCreateShop;
  static readonly GetShopStats: VeggieShopServiceGetShopStats;
  static readonly BuildPurchase: VeggieShopServiceBuildPurchase;
  static readonly MonitorShop: VeggieShopServiceMonitorShop;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class VeggieShopServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  createShop(
    requestMessage: gotestapi_v1_gotestapi_pb.CreateShopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: gotestapi_v1_gotestapi_pb.CreateShopResponse|null) => void
  ): UnaryResponse;
  createShop(
    requestMessage: gotestapi_v1_gotestapi_pb.CreateShopRequest,
    callback: (error: ServiceError|null, responseMessage: gotestapi_v1_gotestapi_pb.CreateShopResponse|null) => void
  ): UnaryResponse;
  getShopStats(
    requestMessage: gotestapi_v1_gotestapi_pb.GetStatsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: gotestapi_v1_gotestapi_pb.GetStatsResponse|null) => void
  ): UnaryResponse;
  getShopStats(
    requestMessage: gotestapi_v1_gotestapi_pb.GetStatsRequest,
    callback: (error: ServiceError|null, responseMessage: gotestapi_v1_gotestapi_pb.GetStatsResponse|null) => void
  ): UnaryResponse;
  buildPurchase(metadata?: grpc.Metadata): RequestStream<gotestapi_v1_gotestapi_pb.BuildPurchaseRequest>;
  monitorShop(requestMessage: gotestapi_v1_gotestapi_pb.MonitorShopRequest, metadata?: grpc.Metadata): ResponseStream<gotestapi_v1_gotestapi_pb.MonitorShopResponse>;
}

