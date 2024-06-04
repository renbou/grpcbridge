// package: gotestapi.v1
// file: gotestapi/v1/gotestapi.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../google/api/annotations_pb";
import * as gotestapi_v1alpha_gotestapi_pb from "../../gotestapi/v1alpha/gotestapi_pb";

export class QuantifiedVeggie extends jspb.Message {
  getVeggie(): gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap];
  setVeggie(value: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap]): void;

  getQuantity(): number;
  setQuantity(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): QuantifiedVeggie.AsObject;
  static toObject(includeInstance: boolean, msg: QuantifiedVeggie): QuantifiedVeggie.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: QuantifiedVeggie, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): QuantifiedVeggie;
  static deserializeBinaryFromReader(message: QuantifiedVeggie, reader: jspb.BinaryReader): QuantifiedVeggie;
}

export namespace QuantifiedVeggie {
  export type AsObject = {
    veggie: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap],
    quantity: number,
  }
}

export class CreateShopRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateShopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateShopRequest): CreateShopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateShopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateShopRequest;
  static deserializeBinaryFromReader(message: CreateShopRequest, reader: jspb.BinaryReader): CreateShopRequest;
}

export namespace CreateShopRequest {
  export type AsObject = {
  }
}

export class CreateShopResponse extends jspb.Message {
  getShopId(): string;
  setShopId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateShopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CreateShopResponse): CreateShopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateShopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateShopResponse;
  static deserializeBinaryFromReader(message: CreateShopResponse, reader: jspb.BinaryReader): CreateShopResponse;
}

export namespace CreateShopResponse {
  export type AsObject = {
    shopId: string,
  }
}

export class GetStatsRequest extends jspb.Message {
  getShopId(): string;
  setShopId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetStatsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetStatsRequest): GetStatsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetStatsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetStatsRequest;
  static deserializeBinaryFromReader(message: GetStatsRequest, reader: jspb.BinaryReader): GetStatsRequest;
}

export namespace GetStatsRequest {
  export type AsObject = {
    shopId: string,
  }
}

export class GetStatsResponse extends jspb.Message {
  clearPurchasedVeggiesList(): void;
  getPurchasedVeggiesList(): Array<QuantifiedVeggie>;
  setPurchasedVeggiesList(value: Array<QuantifiedVeggie>): void;
  addPurchasedVeggies(value?: QuantifiedVeggie, index?: number): QuantifiedVeggie;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetStatsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetStatsResponse): GetStatsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetStatsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetStatsResponse;
  static deserializeBinaryFromReader(message: GetStatsResponse, reader: jspb.BinaryReader): GetStatsResponse;
}

export namespace GetStatsResponse {
  export type AsObject = {
    purchasedVeggiesList: Array<QuantifiedVeggie.AsObject>,
  }
}

export class BuildPurchaseRequest extends jspb.Message {
  getShopId(): string;
  setShopId(value: string): void;

  hasAdd(): boolean;
  clearAdd(): void;
  getAdd(): gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap];
  setAdd(value: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap]): void;

  hasRemove(): boolean;
  clearRemove(): void;
  getRemove(): gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap];
  setRemove(value: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap]): void;

  getActionCase(): BuildPurchaseRequest.ActionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BuildPurchaseRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BuildPurchaseRequest): BuildPurchaseRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BuildPurchaseRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BuildPurchaseRequest;
  static deserializeBinaryFromReader(message: BuildPurchaseRequest, reader: jspb.BinaryReader): BuildPurchaseRequest;
}

export namespace BuildPurchaseRequest {
  export type AsObject = {
    shopId: string,
    add: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap],
    remove: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap],
  }

  export enum ActionCase {
    ACTION_NOT_SET = 0,
    ADD = 2,
    REMOVE = 3,
  }
}

export class BuildPurchaseResponse extends jspb.Message {
  clearPurchasedVeggiesList(): void;
  getPurchasedVeggiesList(): Array<QuantifiedVeggie>;
  setPurchasedVeggiesList(value: Array<QuantifiedVeggie>): void;
  addPurchasedVeggies(value?: QuantifiedVeggie, index?: number): QuantifiedVeggie;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BuildPurchaseResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BuildPurchaseResponse): BuildPurchaseResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BuildPurchaseResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BuildPurchaseResponse;
  static deserializeBinaryFromReader(message: BuildPurchaseResponse, reader: jspb.BinaryReader): BuildPurchaseResponse;
}

export namespace BuildPurchaseResponse {
  export type AsObject = {
    purchasedVeggiesList: Array<QuantifiedVeggie.AsObject>,
  }
}

export class MonitorShopRequest extends jspb.Message {
  getShopId(): string;
  setShopId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MonitorShopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MonitorShopRequest): MonitorShopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MonitorShopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MonitorShopRequest;
  static deserializeBinaryFromReader(message: MonitorShopRequest, reader: jspb.BinaryReader): MonitorShopRequest;
}

export namespace MonitorShopRequest {
  export type AsObject = {
    shopId: string,
  }
}

export class MonitorShopResponse extends jspb.Message {
  getUser(): string;
  setUser(value: string): void;

  hasAdd(): boolean;
  clearAdd(): void;
  getAdd(): gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap];
  setAdd(value: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap]): void;

  hasRemove(): boolean;
  clearRemove(): void;
  getRemove(): gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap];
  setRemove(value: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap]): void;

  hasPurchase(): boolean;
  clearPurchase(): void;
  getPurchase(): MonitorShopResponse.Purchase | undefined;
  setPurchase(value?: MonitorShopResponse.Purchase): void;

  getActionCase(): MonitorShopResponse.ActionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MonitorShopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MonitorShopResponse): MonitorShopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MonitorShopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MonitorShopResponse;
  static deserializeBinaryFromReader(message: MonitorShopResponse, reader: jspb.BinaryReader): MonitorShopResponse;
}

export namespace MonitorShopResponse {
  export type AsObject = {
    user: string,
    add: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap],
    remove: gotestapi_v1alpha_gotestapi_pb.VeggieMap[keyof gotestapi_v1alpha_gotestapi_pb.VeggieMap],
    purchase?: MonitorShopResponse.Purchase.AsObject,
  }

  export class Purchase extends jspb.Message {
    clearVeggiesList(): void;
    getVeggiesList(): Array<QuantifiedVeggie>;
    setVeggiesList(value: Array<QuantifiedVeggie>): void;
    addVeggies(value?: QuantifiedVeggie, index?: number): QuantifiedVeggie;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Purchase.AsObject;
    static toObject(includeInstance: boolean, msg: Purchase): Purchase.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Purchase, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Purchase;
    static deserializeBinaryFromReader(message: Purchase, reader: jspb.BinaryReader): Purchase;
  }

  export namespace Purchase {
    export type AsObject = {
      veggiesList: Array<QuantifiedVeggie.AsObject>,
    }
  }

  export enum ActionCase {
    ACTION_NOT_SET = 0,
    ADD = 2,
    REMOVE = 3,
    PURCHASE = 4,
  }
}

