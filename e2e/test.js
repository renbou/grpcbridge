import WebSocket from "ws";
import EventSource from "eventsource";
import { createChannel, createClient, WebsocketTransport } from "nice-grpc-web";
import { ClockService } from "./rstestapi/v1/rstestapi_pb_service.cjs";
import { VeggieShopService } from "./gotestapi/v1/gotestapi_pb_service.cjs";
import gotestapiv1 from "./gotestapi/v1/gotestapi_pb.cjs";
import gotestapiv1alpha from "./gotestapi/v1alpha/gotestapi_pb.cjs";
import empty from "./empty.cjs";

// REST-like API format via Fetch API
async function restAPIviaFetch(addr) {
  const response = await fetch(`http://${addr}/api/veggieshop/v1/shops`, {
    method: "POST",
  }).then((r) => r.json());

  if (!response.shopId) {
    throw new Error("shopId is not set");
  }

  return response.shopId;
}

// REST-like API format via WebSocket
function restAPIviaWebSocket(addr) {
  const EVENTS = [
    { motionDetected: {} },
    { doorOpened: true },
    { temperature: 24.1 },
    { doorOpened: false },
    { temperature: 25 },
  ];

  const expected = new Set(EVENTS.map(JSON.stringify));

  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://${addr}/api/iot/v1:ws`);

    let complete = false;

    ws.onopen = () => {
      console.log("\tWebSocket connection opened, running messages");

      for (const event of EVENTS) {
        ws.send(JSON.stringify({ event }));
      }
    };

    ws.onclose = () => {
      console.log("\tWebSocket connection closed");
      resolve(complete);
    };

    ws.onerror = (err) => {
      console.error(`\tWebSocket error: ${err}`);
      reject(err);
    };

    ws.onmessage = (event) => {
      console.log(`\tReceived WS message: ${event.data}`);

      if (typeof event.data !== "string") {
        reject("event.data is not a string");
        return;
      }

      const data = JSON.parse(event.data);
      if (!data.event) {
        reject("event.data doesn't have event");
        return;
      }

      const stringified = JSON.stringify(data.event);
      if (expected.has(stringified)) {
        expected.delete(stringified);
      }

      if (expected.size === 0) {
        complete = true;
        ws.close();
      }
    };
  });
}

// REST-like API format via EventSource
function restAPIviaEventSource(addr, shopID) {
  return new Promise((resolve, reject) => {
    const es = new EventSource(
      `http://${addr}/api/veggieshop/v1/shop/${shopID}:monitorws`
    );

    let total = 0;

    es.onmessage = (event) => {
      console.log(`\tReceived ES message: ${event.data}`);

      total++;

      if (total === 5) {
        resolve(true);
        es.close();
      }
    };

    es.onerror = (err) => {
      console.error(`\tEventSource error: ${JSON.stringify(err)}`);
      reject(err);
    };
  });
}

// gRPC-Web API via Fetch
async function gRPCWebAPIviaFetch(addr) {
  const channel = createChannel(`http://${addr}`);
  const client = createClient(ClockService, channel);
  const resp = await client.getTime(new empty.Empty());
  console.log(
    `\tRetrieved timestamp via gRPC-Web: ${resp.toDate().toISOString()}`
  );
}

// gRPC-Web API via WebSockets
async function gRPCWebAPIviaWebSockets(addr, shopID) {
  const requests = [
    { add: gotestapiv1alpha.Veggie.CARROT },
    { add: gotestapiv1alpha.Veggie.CARROT },
    { add: gotestapiv1alpha.Veggie.TOMATO },
    { remove: gotestapiv1alpha.Veggie.TOMATO },
    { add: gotestapiv1alpha.Veggie.CUCUMBER },
  ];

  const channel = createChannel(`ws://${addr}`, WebsocketTransport());
  const client = createClient(VeggieShopService, channel);

  async function* createRequests() {
    for (const tmpl of requests) {
      const request = new gotestapiv1.BuildPurchaseRequest();
      request.setShopId(shopID);

      if (tmpl.add) {
        request.setAdd(tmpl.add);
      } else {
        request.setRemove(tmpl.remove);
      }

      yield request;
    }
  }

  const veggieNumToStr = Object.fromEntries(
    Object.entries(gotestapiv1alpha.Veggie).map(([k, v]) => [v, k])
  );

  const response = await client.buildPurchase(createRequests());
  for (const veggie of response.getPurchasedVeggiesList()) {
    if (veggie.getQuantity() > 0) {
      console.log(
        `\tBought: ${
          veggieNumToStr[veggie.getVeggie()]
        } - ${veggie.getQuantity()}`
      );
    }
  }
}

async function main() {
  const addr = process.env.ADDR || "localhost:22222";

  console.log("1. Testing REST API via Fetch using gotestapi");

  let veggieShopID = "";
  try {
    veggieShopID = await restAPIviaFetch(addr);
  } catch (err) {
    console.error(`REST API via Fetch test failed: ${err}`);
    return 1;
  }

  console.log(`\tgotestapi Veggie Shop ID: ${veggieShopID}`);
  console.log("2. Testing REST API via WebSocket using pytestapi");

  try {
    const ok = await restAPIviaWebSocket(addr);
    if (!ok) {
      throw "WebSocket test didn't complete properly";
    }
  } catch (err) {
    console.error(`REST API via WebSocket test failed: ${err}`);
    return 1;
  }

  console.log("3. Testing REST API via EventSource using gotestapi");
  try {
    const ok = await restAPIviaEventSource(addr, veggieShopID);
    if (!ok) {
      throw "EventSource test didn't complete properly";
    }
  } catch (err) {
    console.error(`REST API via EventSource test failed: ${err}`);
    return 1;
  }

  console.log("4. Testing gRPC-Web API via Fetch using rstestapi");
  try {
    await gRPCWebAPIviaFetch(addr);
  } catch (err) {
    console.error(`gRPC-Web API via Fetch test failed: ${err}`);
    return 1;
  }

  console.log("5. Testing gRPC-Web API via WebSockets using gotestapi");
  try {
    await gRPCWebAPIviaWebSockets(addr, veggieShopID);
  } catch (err) {
    console.error(`gRPC-Web API via WebSockets test failed: ${err}`);
    return 1;
  }

  return 0;
}

main().then((code) => {
  process.exit(code);
});
