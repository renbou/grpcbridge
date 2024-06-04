// package: gotestapi.v1alpha
// file: gotestapi/v1alpha/gotestapi.proto

import * as gotestapi_v1alpha_gotestapi_pb from "../../gotestapi/v1alpha/gotestapi_pb";
import {grpc} from "@improbable-eng/grpc-web";

type VeggieShopCreatePurchase = {
  readonly methodName: string;
  readonly service: typeof VeggieShop;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof gotestapi_v1alpha_gotestapi_pb.CreatePurchaseRequest;
  readonly responseType: typeof gotestapi_v1alpha_gotestapi_pb.CreatePurchaseResponse;
};

export class VeggieShop {
  static readonly serviceName: string;
  static readonly CreatePurchase: VeggieShopCreatePurchase;
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

export class VeggieShopClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  createPurchase(
    requestMessage: gotestapi_v1alpha_gotestapi_pb.CreatePurchaseRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: gotestapi_v1alpha_gotestapi_pb.CreatePurchaseResponse|null) => void
  ): UnaryResponse;
  createPurchase(
    requestMessage: gotestapi_v1alpha_gotestapi_pb.CreatePurchaseRequest,
    callback: (error: ServiceError|null, responseMessage: gotestapi_v1alpha_gotestapi_pb.CreatePurchaseResponse|null) => void
  ): UnaryResponse;
}

