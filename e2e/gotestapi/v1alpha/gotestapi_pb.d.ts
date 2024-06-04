// package: gotestapi.v1alpha
// file: gotestapi/v1alpha/gotestapi.proto

import * as jspb from "google-protobuf";

export class CreatePurchaseRequest extends jspb.Message {
  getVeggie(): VeggieMap[keyof VeggieMap];
  setVeggie(value: VeggieMap[keyof VeggieMap]): void;

  getQuantity(): number;
  setQuantity(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreatePurchaseRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreatePurchaseRequest): CreatePurchaseRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreatePurchaseRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreatePurchaseRequest;
  static deserializeBinaryFromReader(message: CreatePurchaseRequest, reader: jspb.BinaryReader): CreatePurchaseRequest;
}

export namespace CreatePurchaseRequest {
  export type AsObject = {
    veggie: VeggieMap[keyof VeggieMap],
    quantity: number,
  }
}

export class CreatePurchaseResponse extends jspb.Message {
  getPurchaseId(): string;
  setPurchaseId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreatePurchaseResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CreatePurchaseResponse): CreatePurchaseResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreatePurchaseResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreatePurchaseResponse;
  static deserializeBinaryFromReader(message: CreatePurchaseResponse, reader: jspb.BinaryReader): CreatePurchaseResponse;
}

export namespace CreatePurchaseResponse {
  export type AsObject = {
    purchaseId: string,
  }
}

export interface VeggieMap {
  UNKNOWN: 0;
  CARROT: 1;
  TOMATO: 2;
  CUCUMBER: 3;
  POTATO: 4;
}

export const Veggie: VeggieMap;

