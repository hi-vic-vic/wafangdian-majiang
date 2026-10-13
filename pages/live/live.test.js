const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
let page
global.Page = value => { page = value }
require('./live')
delete global.Page

function render(seat, overrides = {}) {
  const room = {
    code: '123456', seat, turn: 2, dealer: 0, phase: 'running', wall: 60,
    handNumber: 1, revision: 7, deadline: Date.now() + 30000, votes: [],
    hand: ['1m', '1m', '9p', '7z'], actions: [{type: 'peng', tile: '1m'}, {type: 'pass'}],
    players: Array.from({length: 4}, (_, i) => ({seat: i, name: '牌友' + i, count: 13,
      ready: true, connected: true, score: 0, discards: ['1m'],
      melds: [{type: 'kong', tiles: ['back', '1m', '1m', 'back']}]})), ...overrides
  }
  const instance = {...page, room, data: {...page.data}, setData(value) { Object.assign(this.data, value) }}
  instance.render()
  return instance
}

test('本家始终在下方，其他座位按出牌顺序旋转且不展示手牌', () => {
  for (let seat = 0; seat < 4; seat++) {
    const {data} = render(seat)
    assert.equal(data.players[seat].position, 'bottom')
    assert.equal(data.players[(seat + 1) % 4].position, 'right')
    assert.equal(data.players[(seat + 2) % 4].position, 'top')
    assert.equal(data.players[(seat + 3) % 4].position, 'left')
    assert.equal(data.players[(seat + 1) % 4].backs.length, 13)
    assert.equal(data.players[0].melds[0].tiles[0].image, '')
    assert.deepEqual(data.actions.map(a => a.index), [0, 1])
  }
})

test('34 种牌映射到上游资源；摸牌置于右侧且重复牌不会丢失', () => {
  const tiles = require('../../utils/mahjong-engine').TILE_TYPES
  const {data} = render(1, {hand: tiles, drawnTile: '1m'})
  assert.equal(data.hand.length, 34)
  assert.equal(data.hand.at(-1).id, '1m')
  assert.equal(data.hand.at(-1).drawn, true)
  for (const tile of data.hand) assert.ok(fs.existsSync(path.join(__dirname, '../..', tile.image)), tile.image)
  const byId = Object.fromEntries(data.hand.map(tile => [tile.id, tile.image]))
  assert.match(byId['1m'], /MahjongVal_21_@2x.png$/)
  assert.match(byId['9p'], /MahjongVal_89_@2x.png$/)
  assert.match(byId['1s'], /MahjongVal_41_@2x.png$/)
  assert.match(byId['5z'], /MahjongVal_126_@2x.png$/)
  assert.match(byId['6z'], /MahjongVal_188_@2x.png$/)
  assert.match(byId['7z'], /MahjongVal_255_@2x.png$/)
  const instance = render(2, {drawnTile: '1m'})
  assert.equal(instance.data.hand.filter(t => t.id === '1m').length, 2)
  let sent
  instance.send = message => { sent = message }
  instance.select({currentTarget: {dataset: {index: 3}}})
  instance.discard()
  assert.deepEqual(sent, {type: 'discard', tile: '1m'})
})

test('混子标记、开局骰子展示和禁出；吃碰说明显示代牌', () => {
  const instance = render(2, {
    wildTile:'1m', indicator:'9m', drawnTile:'1m',
    opening:{firstDice:[3,4],secondDice:[2,5],wallSeat:0,indicatorStack:7,breakStacks:14},
    actions:[{type:'peng',tile:'3m',tiles:['3m','1m'],faces:['3m','3m']}]
  })
  assert.equal(instance.data.hand.filter(t=>t.wild).length,2)
  assert.equal(instance.data.wildLabel,'1万')
  assert.match(instance.data.openingText,/合计14墩/)
  assert.match(instance.data.actions[0].label,/混代3万/)
  let sent
  instance.send=m=>{sent=m}
  instance.select({currentTarget:{dataset:{index:instance.data.hand.findIndex(t=>t.wild)}}})
  instance.discard()
  assert.equal(sent,undefined)
  assert.equal(instance.data.error,'混子不能打出')
})
