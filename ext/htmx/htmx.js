(function () {
  window.$x = window.$x || {
    /**
     * A global object to manage custom events and callbacks.
     *
     * @property {Function} ready - Registers a callback function to be executed
     * once when the DOM is fully loaded or when an `htmx:load` event occurs.
     *
     * @function ready
     * @param {Function} callback - The callback function to be executed.
     * @param {String} selector - The selector to be used to check if the callback
     *     should be executed.
     */
    ready: function (callback, selector) {
      const f = function (evt) {
        if (selector) {
          if (document.querySelector(selector)) {
            callback(evt);
          }
        } else {
          callback(evt);
        }
      };
      let boosted = false;
      document.addEventListener("DOMContentLoaded", function (evt) {
        f(evt);
      });
      document.addEventListener("htmx:load", function (evt) {
        if (boosted) {
          f(evt);
          boosted = false;
        }
      });
      document.addEventListener("htmx:beforeOnLoad", function (evt) {
        // trigger ready function again when a boosted request is done
        boosted = evt.detail.boosted;
      });
    },
    /**
     * The fetch function is a wrapper of native fetch with Hx-Trigger support
     * like it in htmx requests.
     *
     * @function fetch
     * @async
     * @param {String|Request} input - The URL to be requested or the Request
     *     object.
     * @param {Object} init - The options to be used for the request. See the
     * {@link
     * https://developer.mozilla.org/en-US/docs/Web/API/WindowOrWorkerGlobalScope/fetch|fetch}
     * API.
     * @returns {Promise<Response>} - The response of the request.
     */
    fetch: async (...args) => {
      const response = await fetch(...args);
      if (!response.ok) {
        const hx = response.headers.get("Hx-Trigger");
        if (hx) {
          const d = JSON.parse(hx);
          const keys = Object.keys(d);
          for (const key of keys) {
            window.dispatchEvent(new CustomEvent(key, { detail: d[key] }));
          }
        }
      }
      return response;
    },
  };
})();
