const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const path = require('path');
const app = express();
const port = 8080;

app.use('/api', createProxyMiddleware({
  target: 'http://127.0.0.1:8081',
  changeOrigin: true,
}));

app.use('/files', createProxyMiddleware({
  target: 'http://127.0.0.1:8081',
  changeOrigin: true,
}));

app.use(express.static(path.join(__dirname, 'frontend/dist')));

app.get('*', (req, res) => {
  res.sendFile(path.join(__dirname, 'frontend/dist/index.html'));
});

app.listen(port, () => {
  console.log(`Server running at http://127.0.0.1:${port}/`);
});
