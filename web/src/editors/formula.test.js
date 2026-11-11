import assert from 'node:assert/strict'
import test from 'node:test'
import { displayCell } from './formula.js'

function model(cells) {
  return { cells }
}

test('arithmetic and cell references', () => {
  const sheet = model([
    { row: 1, col: 1, input: '2' },
    { row: 1, col: 2, input: '3' },
    { row: 2, col: 1, input: '=A1*B1+1' },
  ])
  assert.equal(displayCell(sheet, 2, 1), '7')
})

test('sum average and text is ignored', () => {
  const sheet = model([
    { row: 1, col: 1, input: '10' },
    { row: 2, col: 1, input: '备注' },
    { row: 3, col: 1, input: '30' },
    { row: 4, col: 1, input: '=SUM(A1:A3)' },
    { row: 5, col: 1, input: '=AVERAGE(A1:A3)' },
    { row: 6, col: 1, input: '=COUNT(A1:A3)' },
  ])
  assert.equal(displayCell(sheet, 4, 1), '40')
  assert.equal(displayCell(sheet, 5, 1), '20')
  assert.equal(displayCell(sheet, 6, 1), '2')
})

test('division by zero and cycles', () => {
  const sheet = model([
    { row: 1, col: 1, input: '=1/0' },
    { row: 2, col: 1, input: '=A3' },
    { row: 3, col: 1, input: '=A2' },
  ])
  assert.equal(displayCell(sheet, 1, 1), '#DIV/0!')
  assert.equal(displayCell(sheet, 2, 1), '#CYCLE!')
})
