import asyncio
import logging
import os
import sys

import structlog
from grpclib.events import RecvRequest, SendTrailingMetadata, listen
from grpclib.reflection.service import ServerReflection
from grpclib.server import Server
from grpclib.utils import graceful_exit
from structlog.stdlib import BoundLogger

from pytestapi import WrapperService

logger: BoundLogger


async def recv_request(event: RecvRequest):
    logger.info("started call", method=event.method_name, peer=event.peer.addr())


async def send_trailing_metadata(event: SendTrailingMetadata):
    logger.info(
        "finished call",
        status_code=event.status,
        status_message=event.status_message,
    )


async def run():
    logging.basicConfig(
        format="%(message)s",
        stream=sys.stdout,
        level=logging.INFO,
    )

    structlog.configure(
        processors=[
            structlog.stdlib.add_logger_name,
            structlog.stdlib.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.StackInfoRenderer(),
            structlog.processors.EventRenamer("msg"),
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.stdlib.BoundLogger,
        logger_factory=structlog.stdlib.LoggerFactory(),
    )

    global logger
    logger = structlog.stdlib.get_logger("pytestapi-server").bind(app="pytestapi")

    listen_addr = ":50052"
    if os.getenv("LISTEN_ADDR") is not None:
        listen_addr = os.getenv("LISTEN_ADDR")

    host, port = listen_addr.split(":")
    host = host or None
    port = port or None

    services = [WrapperService()]
    services = ServerReflection.extend(services)

    server = Server(services)
    listen(server, RecvRequest, recv_request)
    listen(server, SendTrailingMetadata, send_trailing_metadata)

    with graceful_exit([server]):
        await server.start(host=host, port=port)
        logger.info("serving test gRPC", listen_addr=listen_addr)
        await server.wait_closed()


def main():
    asyncio.run(run())
