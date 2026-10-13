const assert = require('assert')
const engine = require('./mahjong-engine')

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`) }
  catch (error) { console.error(`✗ ${name}`); throw error }
}

test('普通 4 组加将可以胡', () => {
  assert.equal(engine.isStandardWin(['1m','2m','3m','2p','3p','4p','5s','6s','7s','1z','1z','1z','5z','5z']), true)
})
test('字牌不能组成顺子', () => {
  assert.equal(engine.isStandardWin(['1m','2m','3m','2p','3p','4p','5s','6s','7s','1z','2z','3z','5z','5z']), false)
})
test('四张同牌可作七小对中的两对', () => {
  assert.equal(engine.isSevenPairs(['1m','1m','1m','1m','2m','2m','3p','3p','4p','4p','1z','1z','7z','7z']), true)
})
test('混可以补成十三幺', () => {
  const hand = ['1m','9m','1p','9p','1s','9s','1z','2z','3z','4z','5z','6z','7z','2m']
  assert.equal(engine.isThirteenOrphans(hand, '2m'), true)
})
test('万筒条翻下一张，九回一', () => {
  for (const suit of ['m', 'p', 's']) {
    for (let rank = 1; rank <= 9; rank++) {
      assert.equal(engine.nextWildTile(`${rank}${suit}`), `${rank % 9 + 1}${suit}`)
    }
  }
})
test('风牌按东西南北循环，北回东', () => {
  const cycle = ['1z', '3z', '2z', '4z']
  cycle.forEach((tile, i) => assert.equal(engine.nextWildTile(tile), cycle[(i + 1) % cycle.length]))
})
test('中发白循环，白回中', () => {
  const cycle = ['5z', '6z', '7z']
  cycle.forEach((tile, i) => assert.equal(engine.nextWildTile(tile), cycle[(i + 1) % cycle.length]))
})
test('拒绝不合法的翻混指示牌', () => {
  for (const tile of ['0m', '10m', '8z', '', null]) assert.throws(() => engine.nextWildTile(tile), /翻混指示牌不合法/)
})
test('手牌按万筒条字牌与点数排序', () => {
  const hand = ['7z', '9s', '1p', '3m', '2z', '1m', '9m', '2p', '1s', '5z']
  assert.deepEqual(engine.sortTiles(hand), ['1m', '3m', '9m', '1p', '2p', '1s', '9s', '2z', '5z', '7z'])
  assert.deepEqual(hand, ['7z', '9s', '1p', '3m', '2z', '1m', '9m', '2p', '1s', '5z'])
})
test('庄家自摸清一色时倍率相乘', () => {
  const result = engine.calculateWinSettlement({ baseScore: 2, dealerWon: true, selfDraw: true, bigHandMultipliers: [2] })
  assert.deepEqual(result, { multiplier: 8, perPayer: 16, winnerReceives: 48, paymentMode: '三家各付' })
})
test('荒庄不结算杠分', () => assert.deepEqual(engine.calculateKongSettlement([{ type: 'concealed', owner: 1 }], false), { transfers: [], total: 0 }))
test('有人胡时暗杠由其余三家各付十分', () => {
  const result = engine.calculateKongSettlement([{ type: 'concealed', owner: 1 }], true)
  assert.equal(result.transfers.length, 3)
  assert.equal(result.total, 30)
})
