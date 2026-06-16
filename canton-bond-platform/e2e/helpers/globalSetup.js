async function globalSetup() {
  const { initFactory, healthCheck } = require('./api.js');

  console.log('Waiting for backend API...');
  let retries = 30;
  while (retries > 0) {
    try {
      await healthCheck();
      console.log('Backend API is healthy');
      break;
    } catch {
      retries--;
      if (retries === 0) {
        throw new Error('Backend API not ready after 60s');
      }
      await new Promise(r => setTimeout(r, 2000));
    }
  }

  console.log('Initializing factory...');
  retries = 10;
  while (retries > 0) {
    try {
      const result = await initFactory();
      console.log(`Factory initialized: ${result.status}`);
      return;
    } catch (err) {
      retries--;
      if (retries === 0) {
        throw new Error(`Factory initialization failed: ${err.message}`);
      }
      console.log(`Retrying factory init (${retries} left)...`);
      await new Promise(r => setTimeout(r, 3000));
    }
  }
}

module.exports = globalSetup;
