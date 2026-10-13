const api = require('../../utils/live');
const engine = require('../../utils/mahjong-engine');
const names = {'1z':'东','2z':'南','3z':'西','4z':'北','5z':'中','6z':'发','7z':'白'};
const label = t => names[t] || t[0] + ({m:'万',p:'筒',s:'条'}[t[1]] || '');
// Face IDs follow upstream MahjongTileDef.ts, not this game's tile encoding.
const honors = [101,103,105,107,126,188,255];
const tile = (t, index) => ({id:t, index, label:t === 'back' ? '暗杠' : label(t),
  image:t === 'back' ? '' : '/assets/whmj/table/MahjongVal_' +
    (t[1] === 'z' ? honors[Number(t[0]) - 1] : ({m:20,p:80,s:40}[t[1]]) + Number(t[0])) + '_@2x.png'});
const positions = ['bottom','right','top','left'];
const actionImages = {hu:'Hu',peng:'Peng',kong:'Gang',add_kong:'Gang',chi:'Chi',pass:'Guo'};

Page({
  data:{room:null,hand:[],players:[],actions:[],status:'连接中',error:'',selected:-1,seconds:0,showMore:false},
  onLoad() {
    this.session = wx.getStorageSync('live-session');
    if (!this.session) { this.setData({error:'请返回首页创建或加入房间'}); return; }
    this.connect();
    this.clock = setInterval(() => {
      if (this.room) this.setData({seconds:Math.max(0,Math.ceil((this.room.deadline-Date.now())/1000))});
    },1000);
  },
  onUnload() {
    this.stopped = true;
    clearTimeout(this.retry); clearInterval(this.clock);
    if (this.socket) this.socket.close();
  },
  connect() {
    if (this.stopped) return;
    this.socket = wx.connectSocket({url:api.base.replace('http','ws')+'/v1/rooms/'+this.session.code+'/ws?token='+this.session.token});
    this.socket.onOpen(() => { this.connected = true; this.setData({error:''}); });
    this.socket.onMessage(e => {
      let v; try { v = JSON.parse(e.data); } catch (_) { return; }
      if (v.type === 'error') { this.setData({error:v.message}); return; }
      if (this.room && v.room.revision < this.room.revision) return;
      this.room = v.room; this.render();
    });
    this.socket.onError(() => this.setData({error:'连接失败，请检查Go服务及域名校验设置'}));
    this.socket.onClose(() => {
      this.connected = false;
      if (this.stopped) return;
      this.setData({error:'连接中断，正在重连；服务重启后需重新建房'});
      this.retry = setTimeout(() => this.connect(),2000);
    });
  },
  render() {
    const r = this.room;
    const players = r.players.map(p => ({...p,
      position:positions[(p.seat-r.seat+4)%4], initial:(p.name || '空').slice(0,1),
      backs:Array.from({length:p.count},(_,i) => i), discards:p.discards.map(tile),
      melds:p.melds.map((m,index) => ({...m,index,tiles:m.tiles.map((t,i)=>({...tile((m.faces||m.tiles)[i],i),wild:t===r.wildTile,originalLabel:label(t)}))})), delta:r.bill ? r.bill.delta[p.seat] : 0
    }));
    const hand = engine.sortTiles(r.hand).map((t,i)=>({...tile(t,i),wild:t===r.wildTile}));
    const drawnIndex = r.drawnTile ? hand.findIndex(t => t.id === r.drawnTile) : -1;
    if (drawnIndex >= 0) hand.push({...hand.splice(drawnIndex,1)[0],drawn:true});
    this.setData({room:r,players,hand,selected:-1,wildLabel:r.wildTile?label(r.wildTile):'',indicatorLabel:r.indicator?label(r.indicator):'',openingText:r.opening?'第一次 '+r.opening.firstDice.join('+')+' · '+r.players[r.opening.wallSeat].name+'第'+r.opening.indicatorStack+'墩上张；第二次 '+r.opening.secondDice.join('+')+'，合计'+r.opening.breakStacks+'墩':'',turnPosition:(r.turn-r.seat+4)%4,
      canDiscard:r.phase === 'running' && r.turn === r.seat,
      canReady:['lobby','settled'].includes(r.phase) && !r.players[r.seat].ready,
      status:({lobby:'等待四人准备',running:'轮到'+r.players[r.turn].name+'出牌',react:'等待吃碰杠胡响应',settled:'本局已结算',closed:'房间已解散'}[r.phase]) +
        (r.drawnTile ? ' · 刚摸 '+label(r.drawnTile) : ''),
      actions:r.actions.map((m,i) => ({...m,index:i,
        image:actionImages[m.type] ? '/assets/whmj/table/Button_'+actionImages[m.type]+'_@2x.png' : '',
        label:({hu:'胡',peng:'碰',kong:'杠',add_kong:'补杠',chi:'吃',pass:'过'}[m.type])+' '+(m.tile ? label(m.tile) : '')+' '+(m.tiles || []).map((t,j)=>t===r.wildTile&&m.faces?'混代'+label(m.faces[j]):label(t)).join(' ')
      }))});
  },
  send(m) {
    if (!this.connected) { this.setData({error:'尚未连接'}); return; }
    this.setData({error:''});
    this.socket.send({data:JSON.stringify({...m,revision:this.room.revision})});
  },
  ready() { this.send({type:'ready'}); },
  select(e) { const i=Number(e.currentTarget.dataset.index);if(this.data.hand[i]&&this.data.hand[i].wild){this.setData({selected:-1,error:'混子不能打出'});return} this.setData({selected:i}); },
  discard() { const t = this.data.hand[this.data.selected]; if (t && !t.wild) this.send({type:'discard',tile:t.id}); },
  action(e) { this.send(this.room.actions[Number(e.currentTarget.dataset.index)]); },
  dissolve() { this.send({type:'dissolve'}); },
  cancel() { this.send({type:'cancel_vote'}); },
  toggleMore() { this.setData({showMore:!this.data.showMore}); },
  back() { wx.reLaunch({url:'/pages/home/home'}); },
  onShareAppMessage() { return {title:'瓦房店推倒胡，房间'+this.session.code,path:'/pages/home/home?room='+this.session.code}; }
});
