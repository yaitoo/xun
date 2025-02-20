window.xun = window.xun || {
  /**
   * A global object to manage custom events and callbacks.
   *
   * @property {Function} ready - Registers a callback function to be executed once when 
   * the DOM is fully loaded or when an `htmx:load` event occurs.
   * 
   * @function ready
   * @param {Function} callback - The callback function to be executed.
   * @param {String} selector - The selector to be used to check if the callback should be executed.
   */
    ready:function(callback,selector){
      const f = function(evt){
       if(selector){
        if(document.querySelector(selector)){
          callback(evt);
        }
       }else{
        callback(evt);
       }
      }
      let boosted = false;
      document.addEventListener('DOMContentLoaded',function(evt){
        f(evt);
      });
      document.addEventListener('htmx:load', function(evt) {
        if(boosted){
          f(evt);
          boosted = false;  
        }
      });  
      document.addEventListener('htmx:beforeOnLoad', function(evt) {
        // trigger ready function again when a boosted request is done
        boosted = evt.detail.boosted;
      });
    }
  }