const { contextBridge } = require('electron')

contextBridge.exposeInMainWorld('allianceDesktop', {
  platform: process.platform,
})
