import { createContext, useState, useEffect } from 'react'
import Router from 'next/router'
import axios from 'axios'
import { useCookies } from 'react-cookie'

const AuthContext = createContext({
  authReady: false,
  cookie: {},
  hasRedirected: false,
  loginError: false,
  providers: [],
  user: null,
  getAccessKey: async (code, providerID, redirectURL) => {},
  login: (selectedIdp) => {},
  logout: async () => {},
  register: async (key) => {}
})

// TODO: need to revisit this - when refresh the page, this get call
const redirectAccountPage = async (currentProviders) => {
  if (currentProviders.length > 0) {
    await Router.push({
      pathname: '/account/login'
    }, undefined, { shallow: true })
  } else {
    await Router.push({
      pathname: '/account/register'
    }, undefined, { shallow: true })
  }
}

export const AuthContextProvider = ({ children }) => {
  const [user, setUser] = useState(null)
  const [hasRedirected, setHasRedirected] = useState(false)
  const [loginError, setLoginError] = useState(false)
  const [authReady, setAuthReady] = useState(false)

  const [providers, setProviders] = useState([])
  const [cookie, setCookie, removeCookies] = useCookies(['accessKey'])

  useEffect(() => {
    const source = axios.CancelToken.source()
    axios.get('/v1/providers')
      .then(async (response) => {
        setProviders(response.data)
        await redirectAccountPage(response.data)
      })
      .catch(() => {
        setLoginError(true)
      })
    return function () {
      source.cancel('Cancelling in cleanup')
    }
  }, [])

  const getCurrentUser = async (key) => {
    return await axios.get('/v1/introspect', { headers: { Authorization: `Bearer ${key}` } })
      .then((response) => {
        return response.data
      })
      .catch(() => {
        setAuthReady(false)
        setLoginError(true)
      })
  }

  const redirectToDashboard = async (key) => {
    try {
      const currentUser = await getCurrentUser(key)

      setUser(currentUser)
      setAuthReady(true)

      await Router.push({
        pathname: '/'
      }, undefined, { shallow: true })
    } catch (error) {
      setLoginError(true)
    }
  }

  const getAccessKey = async (code, providerID, redirectURL) => {
    setHasRedirected(true)
    axios.post('/v1/login', { providerID, code, redirectURL })
      .then(async (response) => {
        setCookie('accessKey', response.data.accessKey, { path: '/' })
        await redirectToDashboard(response.data.accessKey)
      })
      .catch(async () => {
        setAuthReady(false)
        setLoginError(true)
        await Router.push({
          pathname: '/account/login'
        }, undefined, { shallow: true })
      })
  }

  const login = (selectedIdp) => {
    window.localStorage.setItem('providerId', selectedIdp.id)

    const state = [...Array(10)].map(() => (~~(Math.random() * 36)).toString(36)).join('')
    window.localStorage.setItem('state', state)

    const infraRedirect = window.location.origin + '/account/callback'
    window.localStorage.setItem('redirectURL', infraRedirect)

    document.location.href = `https://${selectedIdp.url}/oauth2/v1/authorize?redirect_uri=${infraRedirect}&client_id=${selectedIdp.clientID}&response_type=code&scope=openid+email+groups+offline_access&state=${state}`
  }

  const logout = async () => {
    await axios.post('/v1/logout', {}, { headers: { Authorization: `Bearer ${cookie.accessKey}` } })
      .then(async () => {
        setAuthReady(false)
        setHasRedirected(false)
        await redirectAccountPage(providers)
        removeCookies('accessKey', { path: '/' })
      })
  }

  const register = async (key) => {
    setCookie('accessKey', key, { path: '/' })
    await redirectToDashboard(key)
  }

  const context = {
    authReady,
    cookie,
    hasRedirected,
    loginError,
    providers,
    user,
    getAccessKey,
    login,
    logout,
    register
  }

  return (
    <AuthContext.Provider value={context}>
      {children}
    </AuthContext.Provider>
  )
}

export default AuthContext