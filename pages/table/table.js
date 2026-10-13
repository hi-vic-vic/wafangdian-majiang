const { standardPreset } = require('../../data/rules')

const hand = ['1m', '2m', '3m', '5m', '5m', '7p', '8p', '9p', '2s', '3s', '4s', '1z', '1z', '6z']
const labels = { '1z': '东', '2z': '南', '3z': '西', '4z': '北', '5z': '中', '6z': '发', '7z': '白' }
const imageGroups = { p: 1, m: 2, s: 3, z: 4 }
const suitNames = { m: '万', p: '筒', s: '条' }

Page({
  data: {
    roomCode: '648 219',
    config: standardPreset,
    hand: hand.map((id) => ({ id, label: labels[id] || `${id[0]}${suitNames[id[1]]}`, suit: id[1], image: `/assets/mahjong/handmah_${imageGroups[id[1]]}${id[0]}.png` })),
    selected: -1,
    status: '轮到你摸牌',
    countdown: 26,
    log: ['第 1 圈 · 东家坐庄', '牌局已开始，房间规则已锁定'],
    players: [
      { seat: '上家', name: '阿涛', score: 0, state: '在线' },
      { seat: '对家', name: '海燕', score: 0, state: '在线' },
      { seat: '下家', name: '老程', score: 0, state: '在线' }
    ]
  },
  onLoad() {
    const config = getApp().globalData.roomConfig || standardPreset
    this.setData({ config })
  },
  selectTile(event) { this.setData({ selected: Number(event.currentTarget.dataset.index), status: '已选中一张牌' }) },
  playTile() {
    if (this.data.selected < 0) return wx.showToast({ title: '先选择一张牌', icon: 'none' })
    const card = this.data.hand[this.data.selected]
    this.setData({ selected: -1, status: `你打出了 ${card.label}`, log: this.data.log.concat([`你打出 ${card.label}，等待其他玩家响应`]) })
  },
  showAction(event) {
    const action = event.currentTarget.dataset.action
    wx.showToast({ title: `${action}请求已提交`, icon: 'none' })
  },
  leave() {
    wx.showModal({ title: '发起解散投票', content: '开局后离开需要其余玩家投票同意。', confirmText: '发起投票' })
  }
})
