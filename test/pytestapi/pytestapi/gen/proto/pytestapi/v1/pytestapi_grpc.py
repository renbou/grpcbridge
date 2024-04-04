# Generated by the Protocol Buffers compiler. DO NOT EDIT!
# source: proto/pytestapi/v1/pytestapi.proto
# plugin: grpclib.plugin.main
import abc
import typing

import grpclib.const
import grpclib.client
if typing.TYPE_CHECKING:
    import grpclib.server

import google.protobuf.empty_pb2
import google.protobuf.wrappers_pb2
import google.protobuf.timestamp_pb2
import proto.pytestapi.v1.pytestapi_pb2


class IOTEventsServiceBase(abc.ABC):

    @abc.abstractmethod
    async def StreamEvents(self, stream: 'grpclib.server.Stream[proto.pytestapi.v1.pytestapi_pb2.StreamEventsRequest, proto.pytestapi.v1.pytestapi_pb2.StreamEventsResponse]') -> None:
        pass

    def __mapping__(self) -> typing.Dict[str, grpclib.const.Handler]:
        return {
            '/pytestapi.v1.IOTEventsService/StreamEvents': grpclib.const.Handler(
                self.StreamEvents,
                grpclib.const.Cardinality.STREAM_STREAM,
                proto.pytestapi.v1.pytestapi_pb2.StreamEventsRequest,
                proto.pytestapi.v1.pytestapi_pb2.StreamEventsResponse,
            ),
        }


class IOTEventsServiceStub:

    def __init__(self, channel: grpclib.client.Channel) -> None:
        self.StreamEvents = grpclib.client.StreamStreamMethod(
            channel,
            '/pytestapi.v1.IOTEventsService/StreamEvents',
            proto.pytestapi.v1.pytestapi_pb2.StreamEventsRequest,
            proto.pytestapi.v1.pytestapi_pb2.StreamEventsResponse,
        )
