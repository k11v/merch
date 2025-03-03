import http from "k6/http";
import { check, sleep } from "k6";
import { SharedArray } from "k6/data";
import {
  randomIntBetween,
  randomItem,
} from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

const url = __ENV.APPTEST_URL || "http://127.0.0.1:8080";
const userFile = __ENV.APPTEST_USER_FILE;
if (!userFile || userFile[0] != "/") {
  throw new Error("APPTEST_USER_FILE is empty or not absolute");
}
const authTokenFile = __ENV.APPTEST_AUTH_TOKEN_FILE;
if (!authTokenFile || authTokenFile[0] != "/") {
  throw new Error("APPTEST_AUTH_TOKEN_FILE is empty or not absolute");
}

const users = new SharedArray("users", function () {
  return JSON.parse(open(userFile));
});
const userAuthTokens = new SharedArray("userAuthTokens", function () {
  return JSON.parse(open(authTokenFile));
});

export const options = {
  stages: [
    { duration: "2m", target: 15 },
    { duration: "1m", target: 30 },
    { duration: "1m", target: 0 },
    { duration: "1m", target: 20 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.0001"], // 99.99% of requests should be successful.
    http_req_duration: ["p(90)<50"], // 90% of requests should have a latency of 50ms or less.
  },
};

function randomUserIndex() {
  return randomIntBetween(0, users.length - 1);
}

function randomUserIndexExcept(except) {
  let i = randomIntBetween(0, users.length - 2);
  if (i >= except && except >= 0) {
    i++;
  }
  return i;
}

export default function () {
  const currentUserIndex = randomUserIndex();
  const token = userAuthTokens[currentUserIndex];
  const headers = {
    Authorization: `Bearer ${token}`,
  };

  const x = Math.random();
  if (x < 0.75) {
    // Auth (75%).

    const r = http.get(`${url}/api/info`, { headers: headers });

    check(r, {
      200: (r) => r.status === 200,
    });
  } else if (x < 0.9) {
    // Buy item (15%).

    const itemName = randomItem(["cup", "book", "pen", "socks", "wallet"]);
    const r = http.get(`${url}/api/buy/${itemName}`, { headers: headers });

    check(r, {
      "200 or 400 and not enough coin": (r) =>
        r.status === 200 ||
        (r.status === 400 && r.body.indexOf("not enough coin") !== -1),
    });
  } else {
    // Send coin (10%).

    const toUserIndex = randomUserIndexExcept(currentUserIndex);
    const req = {
      toUser: users[toUserIndex].username,
      amount: randomIntBetween(1, 25),
    };
    const r = http.post(`${url}/api/sendCoin`, JSON.stringify(req), {
      headers: headers,
    });

    check(r, {
      "200 or 400 and not enough coin": (r) =>
        r.status === 200 ||
        (r.status === 400 && r.body.indexOf("not enough coin") !== -1),
    });
  }

  sleep(0.01);
}
