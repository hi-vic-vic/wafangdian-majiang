const { standardPreset, roomOptions } = require('../../data/rules')

Page({
  data: { config: {...standardPreset, rounds: 0}, roomOptions: {...roomOptions, toggles: roomOptions.toggles.filter(t=>t.key!=='wildcards')}, chosenBigHands: ['七小对', '清一色'], bigChoices: ['十三幺','大三元','大四喜','七小对','清一色','天胡','地胡'].map(name=>({name,checked:['七小对','清一色'].includes(name)})), busy:false },
  chooseRounds(event) { this.setData({ 'config.rounds': Number(event.currentTarget.dataset.value) }) },
  chooseBase(event) { this.setData({ 'config.baseScore': Number(event.currentTarget.dataset.value) }) },
  toggleRule(event) {
    const key = event.currentTarget.dataset.key
    this.setData({ [`config.${key}`]: !this.data.config[key] })
  },
  toggleBigHand(event) {
    const name = event.currentTarget.dataset.name
    const current = this.data.chosenBigHands.slice()
    const index = current.indexOf(name)
    if (index >= 0) current.splice(index, 1); else current.push(name)
    this.setData({ chosenBigHands: current, bigChoices:this.data.bigChoices.map(v=>({...v,checked:current.includes(v.name)})) })
  },
  startRoom() {
    if(this.data.busy)return;
    this.setData({busy:true});
    require('../../utils/live').create(wx.getStorageSync('live-name')||'牌友', {...this.data.config, rounds:0, wildcards:false, bigHands:this.data.config.bigHands?this.data.chosenBigHands:[]})
      .then(()=>wx.redirectTo({url:'/pages/live/live'}))
      .catch(e=>wx.showModal({title:'创建失败',content:e.message,showCancel:false}))
      .finally(()=>this.setData({busy:false}));
  }
})
