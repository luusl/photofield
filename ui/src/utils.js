export function isCloseClick(down, up, timeThreshold, distThreshhold) {
  if (timeThreshold === undefined) {
    timeThreshold = 300;
  }
  if (distThreshhold === undefined) {
    distThreshhold = 5;
  }
  if (!down || !up) return false;
  const duration = up.timeStamp - down.timeStamp;
  const dx = up.screenX - down.screenX;
  const dy = up.screenY - down.screenY;
  const distSquared = dx*dx + dy*dy;
  return duration < timeThreshold && distSquared < distThreshhold*distThreshhold;
}

export async function updateUntilDone(updateFn, continueFn, intervalMs) {
  return new Promise(resolve => {
    if (intervalMs === undefined) intervalMs = 1000;
    async function update() {
      await updateFn();
      if (!continueFn()) {
        resolve();
        return;
      }
      setTimeout(update, intervalMs);
    }
    update();
  })
}

export function throttle(callback, delay) {
  let timer = null;
  let pending = false;
  let pendingThis = null;
  let pendingArgs = null;
  function timeout() {
    timer = null;
    if (pending) {
      pending = false;
      timer = setTimeout(timeout, delay);
      callback.apply(pendingThis, pendingArgs);
    }
  }
  return function() {
    // console.log("throttle", arguments);
    if (timer) {
      pending = true;
      pendingThis = this;
      pendingArgs = arguments;
      return;
    }
    callback.apply(this, arguments);
    timer = setTimeout(timeout, delay);
  };
}

export function waitDebounce(callback, delay) {
  let timer = null;
  let pending = false;
  let pendingThis = null;
  let pendingArgs = null;
  function timeout() {
    timer = null;
    if (pending) {
      pending = false;
      timer = setTimeout(timeout, delay);
      callback.apply(pendingThis, pendingArgs);
    }
  }
  return function() {
    pending = true;
    pendingThis = this;
    pendingArgs = arguments;
    clearTimeout(timer);
    timer = setTimeout(timeout, delay);
  };
}

export function debounce(callback, delay) {
  let timer = null;
  let pending = false;
  let pendingThis = null;
  let pendingArgs = null;
  function timeout() {
    timer = null;
    if (pending) {
      pending = false;
      timer = setTimeout(timeout, delay);
      callback.apply(pendingThis, pendingArgs);
    }
  }
  return function() {
    if (timer) {
      pending = true;
      pendingThis = this;
      pendingArgs = arguments;
      clearTimeout(timer);
      timer = setTimeout(timeout, delay);
      return;
    }
    callback.apply(this, arguments);
    timer = setTimeout(timeout, delay);
  };
}

export function LatestFetcher() {
  let controller = null;
  let latestId = 0;
  const abortError = new Error("Overridden by subsequent request");
  abortError.name = "AbortError";
  return async function fetchLatest(url, options) {
    latestId++;
    const id = latestId;
    if (controller) {
      controller.abort();
    }
    controller = new AbortController();
    if (!options) {
      options = {};
    }
    options.signal = controller.signal;
    const response = await fetch(url, options);
    if (!response.ok) {
      console.warn("Unable to fetch", url, response);
      return null;
    }
    if (id != latestId) throw abortError;
    const json = await response.json();
    if (id != latestId) throw abortError;
    return json;
  }
}