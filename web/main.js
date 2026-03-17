import { createApp } from 'vue'
import App from './App.vue'
import './style.css'

function readJSONScript(id) {
	const el = document.getElementById(id)
	if (!el) {
		return null
	}
	const raw = (el.textContent || '').trim()
	if (!raw) {
		return null
	}
	try {
		return JSON.parse(raw)
	} catch {
		return null
	}
}

createApp(App, {
	bootstrapConfig: readJSONScript('kompass-config'),
	bootstrapData: readJSONScript('kompass-data'),
}).mount('#app')
