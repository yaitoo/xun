 function getCsrfToken() {
  const cookies = document.cookie.split(';');
  for (let cookie of cookies) {
    cookie = cookie.trim();
    const [key, ...valueParts] = cookie.split('=');
    const value = valueParts.join('=');
    if (key === "{{ .CookieName }}") {
      return value;
    }
  }
  return null; 
}

function setCsrfToken() {
  const token = getCsrfToken()
  if (token === null) {
    return
  }
  const name =`js_{{.CookieName}}`;
  document.cookie = `${name}=${token};path=/;samesite=lax`;
}

setCsrfToken();