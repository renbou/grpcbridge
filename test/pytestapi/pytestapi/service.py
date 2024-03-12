from google.protobuf.wrappers_pb2 import BoolValue, FloatValue, Int32Value, StringValue
from grpclib.server import Stream

from .gen.proto.pytestapi.v1.pytestapi_grpc import WrapperServiceBase
from .gen.proto.pytestapi.v1.pytestapi_pb2 import UnaryWrapRequest, UnaryWrapResponse


class WrapperService(WrapperServiceBase):
    async def UnaryWrap(
        self, stream: Stream[UnaryWrapRequest, UnaryWrapResponse]
    ) -> None:
        request = await stream.recv_message()
        response = UnaryWrapResponse(
            string_value=StringValue(value=request.string_value),
            int32_value=Int32Value(value=request.int32_value),
            float_value=FloatValue(value=request.float_value),
            bool_value=BoolValue(value=request.bool_value),
        )
        await stream.send_message(response)
