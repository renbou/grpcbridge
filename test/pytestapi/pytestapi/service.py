import asyncio
from random import choice, random
from time import time

from google.protobuf.empty_pb2 import Empty
from google.protobuf.timestamp_pb2 import Timestamp
from google.protobuf.wrappers_pb2 import BoolValue, FloatValue
from grpclib import Status
from grpclib.server import Stream

from .gen.proto.pytestapi.v1.pytestapi_grpc import IOTEventsServiceBase
from .gen.proto.pytestapi.v1.pytestapi_pb2 import (
    Event,
    StreamEventsRequest,
    StreamEventsResponse,
)

RANDOM_EVENT_INTERVAL = 3
RANDOM_EVENT_JITTER = 0.5


class IOTEventsService(IOTEventsServiceBase):
    async def StreamEvents(
        self, stream: Stream[StreamEventsRequest, StreamEventsResponse]
    ) -> None:
        next_event = time()
        while True:
            try:
                next_event_in = next_event - time()
                req = await asyncio.wait_for(
                    stream.recv_message(), timeout=next_event_in
                )
                if req == None:
                    await stream.send_trailing_metadata(status=Status.OK)
                    return

                if req.event == None or not req.event.HasField("event"):
                    continue
                event = req.event
            except asyncio.TimeoutError:
                event = random_event()
                next_event = time() + (
                    RANDOM_EVENT_INTERVAL + RANDOM_EVENT_JITTER * random()
                )

            resp = StreamEventsResponse(event=event, ts=Timestamp())
            resp.ts.GetCurrentTime()

            await stream.send_message(resp)


def random_event() -> Event:
    match choice(["motion_detected", "door_opened", "temperature"]):
        case "motion_detected":
            return Event(motion_detected=Empty())
        case "door_opened":
            return Event(door_opened=BoolValue(value=choice([True, False])))
        case "temperature":
            return Event(temperature=FloatValue(value=10 + random() * 20))
