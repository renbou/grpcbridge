import os
import sys

# Needed so that the grpclib-generated files can properly import protobufs.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen"))

from .service import IOTEventsService
