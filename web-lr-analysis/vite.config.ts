import {defineConfig} from 'vite'; import {svelte} from '@sveltejs/vite-plugin-svelte'; import {sites} from '@openai/sites-vite-plugin';
export default defineConfig({plugins:[svelte(),sites()],server:{port:5173,proxy:{'/api':'http://127.0.0.1:8080'}}});
