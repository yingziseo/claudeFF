export type Environment = {
  id: string; name: string; note: string; region: string; code: string; ip: string; language: string; timezone: string;
  status: 'running' | 'stopped' | 'unchecked'; tone: string
}

// Isolated design fixtures. These are documentation IP ranges, not network detections.
export function sampleEnvironments(): Environment[] {
  return [
    { id: 'demo-sg', name: '日常工作', note: '一个安静、独立的工作空间', region: '新加坡', code: 'SG', ip: '203.0.113.12', language: '英语', timezone: 'Asia/Singapore', status: 'running', tone: 'blue' },
    { id: 'demo-us', name: '项目研究', note: '保留项目的浏览记录与登录状态', region: '美国', code: 'US', ip: '198.51.100.24', language: '英语', timezone: 'America/Los_Angeles', status: 'stopped', tone: 'purple' },
    { id: 'demo-jp', name: '灵感收集', note: '随时回来，继续上次的工作', region: '日本', code: 'JP', ip: '192.0.2.36', language: '日语', timezone: 'Asia/Tokyo', status: 'stopped', tone: 'amber' },
  ]
}

export const previewNote = '仅供设计体验，示例数据不会进行真实网络检测或启动浏览器。'
