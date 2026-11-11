import assert from 'node:assert/strict'
import test from 'node:test'
import { columnLabel, displayCell, mapFormulaRefs, parseRef } from './formula.js'

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

test('excel functions, absolute refs and columns past Z', () => {
  const workbook = {
    sheets: [
      {
        name: 'Sheet1',
        cells: [
          { row: 1, col: 1, input: '10' },
          { row: 2, col: 1, input: '20' },
          { row: 3, col: 1, input: '苹果' },
          { row: 3, col: 2, input: '红' },
          { row: 1, col: 2, input: '3' },
          { row: 2, col: 2, input: '4' },
          { row: 4, col: 1, input: '=IF(A1>5,"高","低")' },
          { row: 5, col: 1, input: '=IF(FALSE,1/0,"安全")' },
          { row: 6, col: 1, input: '=SUMIF(A1:A2,">15",B1:B2)' },
          { row: 7, col: 1, input: '=VLOOKUP("苹果",A3:B3,2,FALSE)' },
          { row: 8, col: 1, input: '=$A$1' },
          { row: 9, col: 1, input: '=Sheet2!A1' },
          { row: 1, col: 27, input: '=AA1' },
        ],
      },
      { name: 'Sheet2', cells: [{ row: 1, col: 1, input: '跨表' }] },
    ],
  }
  const sheet = workbook.sheets[0]
  assert.equal(displayCell(sheet, 4, 1, workbook), '高')
  assert.equal(displayCell(sheet, 5, 1, workbook), '安全')
  assert.equal(displayCell(sheet, 6, 1, workbook), '4')
  assert.equal(displayCell(sheet, 7, 1, workbook), '红')
  assert.equal(displayCell(sheet, 8, 1, workbook), '10')
  assert.equal(displayCell(sheet, 9, 1, workbook), '跨表')
  assert.equal(displayCell(sheet, 1, 27, workbook), '#CYCLE!')
  assert.equal(columnLabel(27), 'AA')
  assert.deepEqual(parseRef('$B$12'), { col: 2, row: 12, absCol: true, absRow: true })
  assert.equal(mapFormulaRefs('=A1+B$2', (ref) => ({ ...ref, row: ref.absRow ? ref.row : ref.row + 1, col: ref.absCol ? ref.col : ref.col + 1 })), '=B2+C$2')
})
