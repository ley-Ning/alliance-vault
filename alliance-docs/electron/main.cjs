const { app, BrowserWindow, shell } = require('electron')
const path = require('node:path')

const createMainWindow = () => {
  const window = new BrowserWindow({
    width: 1480,
    height: 920,
    minWidth: 1080,
    minHeight: 720,
    backgroundColor: '#efe6d4',
    title: '联盟文舱',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })

  const startUrl = process.env.ELECTRON_START_URL
  if (startUrl) {
    window.loadURL(startUrl)
  } else {
    window.loadFile(path.join(__dirname, '..', 'dist', 'index.html'))
  }

  window.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })
}

app.whenReady().then(() => {
  createMainWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createMainWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})
