import { mount } from 'svelte'
import App from './App.svelte'

// Only mount in browser environment
if (typeof window !== 'undefined') {
  mount(App, {
    target: document.getElementById('app'),
  })
}
