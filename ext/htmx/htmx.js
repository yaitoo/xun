window.$x = window.$x || {
  /**
   * A global object to manage custom events and callbacks.
   *
   * @property {Function} ready - Registers a callback function to be executed once when 
   * the DOM is fully loaded or when an `htmx:load` event occurs.
   * 
   * @function ready
   * @param {Function} fn - The callback function to be executed.
   */
    ready:function(fn){
      let boosted = false;
      document.addEventListener('DOMContentLoaded',function(){
        fn();
      });
      document.addEventListener('htmx:load', function(evt) {
        if(boosted){
          fn(evt);
          boosted = false;  
        }
      });  
      document.addEventListener('htmx:beforeOnLoad', function(evt) {
        // trigger ready function again when a boosted request is done
        boosted = evt.detail.boosted;
      });
    }
  }