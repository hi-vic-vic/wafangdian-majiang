/*
 * 规则引擎只处理确定性的牌型与积分。网络同步、洗牌和权限必须在服务端实现。
 * 牌编码：1m-9m 万，1p-9p 筒，1s-9s 条，1z-7z 东南西北中发白。
 */
const SUITS = ['m', 'p', 's']
const HONORS = ['1z', '2z', '3z', '4z', '5z', '6z', '7z']
const ORPHANS = ['1m', '9m', '1p', '9p', '1s', '9s'].concat(HONORS)

function allTileTypes() {
  const result = []
  SUITS.forEach((suit) => {
    for (let rank = 1; rank <= 9; rank += 1) result.push(`${rank}${suit}`)
  })
  return result.concat(HONORS)
}

const TILE_TYPES = allTileTypes()

function assertHand(hand) {
  if (!Array.isArray(hand) || hand.length !== 14) {
    throw new Error('判胡需要恰好 14 张牌')
  }
  hand.forEach((tile) => {
    if (!TILE_TYPES.includes(tile)) throw new Error(`不支持的牌编码：${tile}`)
  })
}

function countTiles(hand, wildTile) {
  const counts = Object.fromEntries(TILE_TYPES.map((tile) => [tile, 0]))
  let wildCount = 0
  hand.forEach((tile) => {
    if (wildTile && tile === wildTile) wildCount += 1
    else counts[tile] += 1
  })
  return { counts, wildCount }
}

function tileRank(tile) { return Number(tile[0]) }
function tileSuit(tile) { return tile[1] }
function isSuited(tile) { return SUITS.includes(tileSuit(tile)) }

function firstRemaining(counts) {
  return TILE_TYPES.find((tile) => counts[tile] > 0)
}

function memoKey(counts, wild, groups) {
  return `${wild}|${groups}|${TILE_TYPES.map((tile) => counts[tile]).join('')}`
}

function canMakeMelds(counts, wild, groups, memo) {
  const first = firstRemaining(counts)
  if (!first) return wild === groups * 3
  const key = memoKey(counts, wild, groups)
  if (memo.has(key)) return memo.get(key)

  let valid = false
  const tripletTake = Math.min(3, counts[first])
  const tripletNeed = 3 - tripletTake
  if (tripletNeed <= wild) {
    const next = { ...counts, [first]: counts[first] - tripletTake }
    valid = canMakeMelds(next, wild - tripletNeed, groups - 1, memo)
  }

  if (!valid && isSuited(first)) {
    const rank = tileRank(first)
    const suit = tileSuit(first)
    for (let start = Math.max(1, rank - 2); start <= Math.min(rank, 7) && !valid; start += 1) {
      const sequence = [`${start}${suit}`, `${start + 1}${suit}`, `${start + 2}${suit}`]
      const next = { ...counts }
      let need = 0
      sequence.forEach((tile) => {
        if (next[tile] > 0) next[tile] -= 1
        else need += 1
      })
      if (need <= wild) valid = canMakeMelds(next, wild - need, groups - 1, memo)
    }
  }

  memo.set(key, valid)
  return valid
}

function isStandardWin(hand, wildTile) {
  assertHand(hand)
  const { counts, wildCount } = countTiles(hand, wildTile)
  for (const pair of TILE_TYPES) {
    const available = counts[pair]
    const used = Math.min(2, available)
    const neededWild = 2 - used
    if (neededWild > wildCount) continue
    const rest = { ...counts, [pair]: available - used }
    if (canMakeMelds(rest, wildCount - neededWild, 4, new Map())) return true
  }
  return false
}

function isSevenPairs(hand, wildTile) {
  assertHand(hand)
  const { counts, wildCount } = countTiles(hand, wildTile)
  let pairs = 0
  let singles = 0
  for (const tile of TILE_TYPES) {
    pairs += Math.floor(counts[tile] / 2)
    singles += counts[tile] % 2
  }
  if (singles > wildCount) return false
  const remainingWild = wildCount - singles
  return remainingWild % 2 === 0 && pairs + singles + remainingWild / 2 === 7
}

function isThirteenOrphans(hand, wildTile) {
  assertHand(hand)
  const { counts, wildCount } = countTiles(hand, wildTile)
  if (TILE_TYPES.some((tile) => !ORPHANS.includes(tile) && counts[tile] > 0)) return false
  return ORPHANS.some((pairTile) => {
    let needed = 0
    for (const tile of ORPHANS) {
      const target = tile === pairTile ? 2 : 1
      if (counts[tile] > target) return false
      needed += Math.max(0, target - counts[tile])
    }
    return needed === wildCount
  })
}

function isWinningHand(hand, options = {}) {
  const wildTile = options.wildTile || null
  return {
    standard: isStandardWin(hand, wildTile),
    sevenPairs: isSevenPairs(hand, wildTile),
    thirteenOrphans: isThirteenOrphans(hand, wildTile)
  }
}

function nextWildTile(indicator) {
  if (!TILE_TYPES.includes(indicator)) throw new Error('翻混指示牌不合法')
  const suit = tileSuit(indicator)
  const rank = tileRank(indicator)
  if (SUITS.includes(suit)) return `${rank === 9 ? 1 : rank + 1}${suit}`
  // 本地风牌循环为东西南北，编码仍沿用东南西北。
  const honorMap = { '1z': '3z', '3z': '2z', '2z': '4z', '4z': '1z', '5z': '6z', '6z': '7z', '7z': '5z' }
  return honorMap[indicator]
}

function calculateWinSettlement({
  baseScore = 1,
  dealerWon = false,
  selfDraw = false,
  bigHandMultipliers = []
}) {
  if (!Number.isFinite(baseScore) || baseScore <= 0) throw new Error('底分必须为正数')
  const sourceMultiplier = selfDraw ? 2 : 2
  const patternMultiplier = bigHandMultipliers.reduce((total, value) => total * value, 1)
  const multiplier = (dealerWon ? 2 : 1) * sourceMultiplier * patternMultiplier
  const perPayer = baseScore * multiplier
  return {
    multiplier,
    perPayer,
    winnerReceives: selfDraw ? perPayer * 3 : perPayer,
    paymentMode: selfDraw ? '三家各付' : '点炮者单付'
  }
}

function calculateKongSettlement(gangs, endedByWin, playerCount = 4) {
  if (!endedByWin) return { transfers: [], total: 0 }
  const transfers = []
  gangs.forEach((gang) => {
    const perPayer = gang.type === 'concealed' ? 10 : 5
    for (let payer = 0; payer < playerCount; payer += 1) {
      if (payer !== gang.owner) transfers.push({ from: payer, to: gang.owner, score: perPayer })
    }
  })
  return { transfers, total: transfers.reduce((sum, item) => sum + item.score, 0) }
}

function sortTiles(tiles) {
  const suitOrder = { m: 0, p: 1, s: 2, z: 3 }
  return [...tiles].sort((left, right) => {
    const suitDifference = suitOrder[left[1]] - suitOrder[right[1]]
    return suitDifference || Number(left[0]) - Number(right[0])
  })
}

module.exports = {
  TILE_TYPES,
  ORPHANS,
  isStandardWin,
  isSevenPairs,
  isThirteenOrphans,
  isWinningHand,
  nextWildTile,
  sortTiles,
  calculateWinSettlement,
  calculateKongSettlement
}
