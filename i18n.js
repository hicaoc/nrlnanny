(() => {
  const messages = {
    zh: {
      dashboard: '控制台', live: '直播', liveMult: '多房间直播', browser: '录音浏览',
      systemReady: '系统就绪', centerStage: '播放控制', playlist: '播放列表', statistics: '状态信息',
      nextBeacon: '下次信标', currentTask: '当前任务', scanning: '扫描中…', controls: '控制选项',
      micCapture: '麦克风采集', voiceRecording: '语音录音', opusTx: 'Opus 发射 (16 kHz)', beaconCron: '定时信标',
      timePlayback: '整点报时', duckScale: '闪避比例', musicDucking: '音乐闪避', micDucking: '麦克风闪避',
      previous: '上一首', playPause: '播放/暂停', next: '下一首',
      previousStation: '上一个电台', radioPlayPause: '播放/停止电台', nextStation: '下一个电台',
      currentStation: '当前电台', noStationSelected: '未选择电台',
      connecting: '正在连接', connected: '已连接', reconnecting: '正在重连', timeoutRetry: '超时重试',
      mute: '静音', unmute: '取消静音', broadcastTopic: '直播主题', liveBroadcast: '实时直播', standby: '待机',
      commLog: '通话记录', waitingTransmissions: '等待通话…', startListening: '开始监听', tapResumeAudio: '点击恢复音频',
      browserTitle: '录音文件浏览器', loading: '加载中…', loadingDirs: '加载日期列表…', noDirs: '暂无录音目录',
      selectDate: '选择日期目录', loadDirsFailed: '加载目录失败', loadingFiles: '加载录音文件…', backDates: '← 返回日期列表',
      directory: '目录', audioUnsupported: '您的浏览器不支持 audio 标签。', loadFilesFailed: '加载文件失败',
      multiMonitor: '多房间监听', multiTitle: '多房间实时直播', multiIntro: '点击房间卡片开始监听；再次点击即可取消。支持同时订阅多个房间。',
      rooms: '房间', activeCalls: '通话中', listening: '监听中', searchRooms: '搜索房间名称或编号…', volume: '音量',
      loadingRooms: '正在获取房间列表…', recentCalls: '最近 20 次通话', noCalls: '暂无通话记录', noMatchingRooms: '没有匹配的房间',
      noRooms: '暂无可用房间', unnamedRoom: '未命名房间', idle: '空闲', clickListen: '○ 点击监听', nowListening: '● 正在监听',
      wsNotConnected: 'WebSocket 尚未连接', audioUnsupportedShort: '当前浏览器不支持音频播放', configFailed: '配置加载失败', configLoadFailed: '无法加载直播配置：{error}',
      localMusic: '本地音乐', networkRadio: '网络电台', radioName: '电台名称', radioURL: 'MP3 / M3U8 网络地址', radioSave: '收藏电台', radioUpdate: '保存修改', radioCancel: '取消',
      radioPlay: '播放', radioStop: '停止电台', radioEdit: '编辑', radioDelete: '删除', radioEmpty: '还没有收藏网络电台', radioDeleteConfirm: '确定删除这个电台吗？',
      radioStopped: '已停止', radioConnecting: '正在连接', radioPlaying: '正在转发', radioReconnecting: '正在重连', radioRequestFailed: '网络电台操作失败',
      deviceIdentity: '当前保姆：呼号-SSID',
      serverIdentity: '当前连接的 NRL 服务器',
      login: '登录', controlLogin: '控制台登录', username: '用户名', password: '密码',
      signIn: '登录', logout: '退出登录', invalidCredentials: '用户名或密码错误',
      language: 'English'
    },
    en: {
      dashboard: 'DASHBOARD', live: 'LIVE', liveMult: 'LIVE MULT', browser: 'BROWSER',
      systemReady: 'SYSTEM READY', centerStage: 'Center Stage', playlist: 'Playlist', statistics: 'Statistics',
      nextBeacon: 'Next Beacon', currentTask: 'Current Task', scanning: 'Scanning…', controls: 'Controls',
      micCapture: 'Mic Capture', voiceRecording: 'Voice Recording', opusTx: 'Opus TX (16 kHz)', beaconCron: 'Beacon Cron',
      timePlayback: 'Time Playback', duckScale: 'Duck Scale', musicDucking: 'Music Ducking', micDucking: 'Mic Ducking',
      previous: 'Previous', playPause: 'Play/Pause', next: 'Next',
      previousStation: 'Previous station', radioPlayPause: 'Play/Stop radio', nextStation: 'Next station',
      currentStation: 'Current station', noStationSelected: 'No station selected',
      connecting: 'CONNECTING', connected: 'CONNECTED', reconnecting: 'RECONNECTING', timeoutRetry: 'TIMEOUT-RETRY',
      mute: 'MUTE', unmute: 'UNMUTE', broadcastTopic: 'BROADCAST TOPIC', liveBroadcast: 'Live Broadcast', standby: 'STANDBY',
      commLog: 'Comm Log', waitingTransmissions: 'Waiting for transmissions…', startListening: 'START LISTENING', tapResumeAudio: 'TAP TO RESUME AUDIO',
      browserTitle: 'Recordings Browser', loading: 'Loading…', loadingDirs: 'Loading date directories…', noDirs: 'No recording directories',
      selectDate: 'Select a date directory', loadDirsFailed: 'Failed to load directories', loadingFiles: 'Loading recordings…', backDates: '← Back to date directories',
      directory: 'Directory', audioUnsupported: 'Your browser does not support the audio element.', loadFilesFailed: 'Failed to load files',
      multiMonitor: 'MULTI-ROOM MONITOR', multiTitle: 'Multi-room live monitor', multiIntro: 'Click a room to listen; click it again to stop. You can subscribe to multiple rooms at once.',
      rooms: 'Rooms', activeCalls: 'Active calls', listening: 'Listening', searchRooms: 'Search room name or number…', volume: 'Volume',
      loadingRooms: 'Loading room list…', recentCalls: 'Last 20 calls', noCalls: 'No call records', noMatchingRooms: 'No matching rooms',
      noRooms: 'No rooms available', unnamedRoom: 'Unnamed room', idle: 'Idle', clickListen: '○ CLICK TO LISTEN', nowListening: '● LISTENING',
      wsNotConnected: 'WebSocket is not connected', audioUnsupportedShort: 'Your browser does not support audio playback', configFailed: 'Configuration failed', configLoadFailed: 'Could not load live configuration: {error}',
      localMusic: 'Local Music', networkRadio: 'Internet Radio', radioName: 'Station name', radioURL: 'MP3 / M3U8 stream URL', radioSave: 'Save station', radioUpdate: 'Save changes', radioCancel: 'Cancel',
      radioPlay: 'Play', radioStop: 'Stop radio', radioEdit: 'Edit', radioDelete: 'Delete', radioEmpty: 'No saved internet radio stations', radioDeleteConfirm: 'Delete this station?',
      radioStopped: 'Stopped', radioConnecting: 'Connecting', radioPlaying: 'Forwarding', radioReconnecting: 'Reconnecting', radioRequestFailed: 'Internet radio request failed',
      deviceIdentity: 'Current nanny: callsign-SSID',
      serverIdentity: 'Connected NRL server',
      login: 'Login', controlLogin: 'Control panel login', username: 'Username', password: 'Password',
      signIn: 'Sign in', logout: 'Sign out', invalidCredentials: 'Invalid username or password',
      language: '中文'
    }
  };
  let language = localStorage.getItem('nrlnanny-language');
  if (language !== 'zh' && language !== 'en') language = navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
  function t(key, values = {}) {
    let value = (messages[language] && messages[language][key]) || messages.en[key] || key;
    return value.replace(/\{(\w+)\}/g, (_, name) => values[name] ?? '');
  }
  function apply(root = document) {
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en';
    document.querySelectorAll('[data-i18n]').forEach(el => { el.textContent = t(el.dataset.i18n); });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => { el.placeholder = t(el.dataset.i18nPlaceholder); });
    document.querySelectorAll('[data-i18n-title]').forEach(el => { el.title = t(el.dataset.i18nTitle); });
    document.querySelectorAll('[data-language-toggle]').forEach(el => { el.textContent = t('language'); el.setAttribute('aria-label', t('language')); });
    document.title = `NRL Nanny - ${t(document.body.dataset.pageTitle || 'dashboard')}`;
  }
  async function loadDeviceIdentity() {
    try {
      const response = await fetch('/api/status', { cache: 'no-store' });
      if (!response.ok) return;
      const state = await response.json();
      const authorized = state.control_enabled !== false && state.authenticated === true;
      document.querySelectorAll('[data-auth-link]').forEach(el => { el.hidden = !authorized; });
      document.querySelectorAll('[data-login-link]').forEach(el => { el.hidden = authorized || state.control_enabled === false; });
      const callsign = String(state.callsign || '').trim();
      const identity = callsign ? `${callsign}-${state.ssid}` : '--';
      document.querySelectorAll('[data-device-identity]').forEach(el => { el.textContent = `📻 ${identity}`; });
      const server = String(state.server || '').trim();
      const endpoint = server ? `${server}${state.port ? `:${state.port}` : ''}` : '--';
      document.querySelectorAll('[data-server-identity]').forEach(el => { el.textContent = `🌐 ${endpoint}`; });
    } catch (_) {}
  }
  window.NRLI18n = { t, apply, loadDeviceIdentity, get language() { return language; }, toggle() { language = language === 'zh' ? 'en' : 'zh'; localStorage.setItem('nrlnanny-language', language); apply(); document.dispatchEvent(new CustomEvent('nrlnanny-language-change')); } };
  const initialize = () => { apply(); loadDeviceIdentity(); };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', initialize, { once: true }); else initialize();
})();
