package consts

const MetrikaCounterFileName = "common.js"
const MetrikaCounterScript = `
	"use strict";
	Object.defineProperty(exports, "__esModule", { value: true });
	/** @type {Partial<import("@gravity-ui/nodekit").AppConfig>} */
	const config = {
	  metrikaCounter: [
	  {
	    id: 92831672,
	    defer: true,
	    clickmap: true,
	    trackLinks: true,
	    accurateTrackBounce: true,
	    webvisor: true,
	  },
	 ],
	};
	exports.default = config;
`
