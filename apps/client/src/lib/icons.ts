import { registerIcons } from '@facile/muse';
import data from './icons-data.json';

registerIcons(data);

export const callIcons = {
	mic: 'iconoir:microphone',
	micOff: 'iconoir:microphone-mute',
	cam: 'iconoir:video-camera',
	camOff: 'iconoir:video-camera-off',
	screen: 'iconoir:airplay',
	chat: 'iconoir:chat-lines',
	leave: 'iconoir:phone-disabled',
	record: 'iconoir:circle',
	send: 'iconoir:send',
	link: 'iconoir:link',
	people: 'iconoir:group'
} as const;
