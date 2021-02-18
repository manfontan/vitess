const fs = require('fs')

describe('vitess-mixin: Vtgate Overview Dashboard Test', function() {

  let panelTitles = []

  before(function() {
    cy.readFile('./test/vtgate_overview.json').then((data) => {
      cy.createDashboard(data)
    })
  })
  it('renders vtgate overview dashboard', function() {
    cy.visit('/d/vitess-vtgate-overview/vtgate - overview (auto-generated)')
  })
  //SUB-MENU CONTROLS
  it('contains vitess-mixin Dashboard links dropdown', function() {
    cy.get('.submenu-controls').get('.gf-form').contains('vitess-mixin');
  })
  // RED ROW
  it('contains the RED row', function() {
    cy.get('.dashboard-row').contains('RED - Requests / Error rate / Duration');
  })
  it('contains the Requests panel',function() {
    cy.get('.panel-title').contains('Requests');
  })
  it('contains the Error rate panel',function() {
    cy.get('.panel-title').contains('Error rate');
  })
  it('contains the Duration p99 panel',function() {
    cy.get('.panel-title').contains('Duration 99th quantile');
  })
  it('collapses the RED row', function(){
    cy.get('.dashboard-row__title.pointer').contains('RED - Requests / Error rate / Duration').click();
    cy.get('.dashboard-row--collapsed').contains('RED - Requests / Error rate / Duration');
  })
  // RED (BY KEYSPACE) ROW
  it('contains the RED (by keyspace) row', function() {
    cy.get('.dashboard-row').contains('RED (by keyspace)');
  })
  it('RED (by keyspace) row is collapsed', function() {
    cy.get('.dashboard-row--collapsed').contains('RED (by keyspace)');
  })
  it('RED(by keyspace) row has 3 panels', function(){
    cy.get('.dashboard-row').contains('RED (by keyspace)').find('.dashboard-row__panel_count').contains('(3 panels)');
  })
  it('expands the RED (by keyspace) row', function(){
    cy.get('.dashboard-row__title.pointer').contains('RED (by keyspace)').click();
  })
  it('contains the Requests (by keyspace) panel', function(){
    cy.get('.panel-title').contains('Requests (by keyspace)');
  })
  it('contains the Error rate (by keyspace) panel', function(){
    cy.get('.panel-title').contains('Error rate (by keyspace)');
  })
  it('contains the Duration 99th quantile (by keyspace) panel', function(){
    cy.get('.panel-title').contains('Duration 99th quantile (by keyspace)');
  })
  it('collapses the RED (by keyspace) row', function(){
    cy.get('.dashboard-row__title.pointer').contains('RED (by keyspace)').click();
    cy.get('.dashboard-row--collapsed').contains('RED (by keyspace)');
  })
  // ROW (BY TABLET TYPE)
  it('contains the  RED (by tablet type) row', function() {
    cy.get('.dashboard-row').contains('RED (by tablet type)');
  })
  it('RED (by tablet type) row is collapsed', function() {
    cy.get('.dashboard-row--collapsed').contains('RED (by tablet type)');
  })
  it('RED(by tablet type) row has 3 panels', function(){
    cy.get('.dashboard-row').contains('RED (by tablet type)').find('.dashboard-row__panel_count').contains('(3 panels)');
  })
  it('expands the RED (by tablet type) row', function(){
    cy.get('.dashboard-row__title.pointer').contains('RED (by tablet type)').click();
  })
  it('contains the Requests (by db_type) panel', function(){
    cy.get('.panel-title').contains('Requests (by db_type)');
  })
  it('contains the Error rate (by db_type) panel', function(){
    cy.get('.panel-title').contains('Error rate (by db_type)');
  })
  it('contains the Duration 99th quantile (by db_type)) panel', function(){
    cy.get('.panel-title').contains('Duration 99th quantile (by db_type)');
  })
  it('collapses the RED (by tablet type) row', function(){
    cy.get('.dashboard-row__title.pointer').contains('RED (by tablet type)').click();
    cy.get('.dashboard-row--collapsed').contains('RED (by tablet type)');
  })
  //ERRORS ROW
  it('contains the Errors row', function() {
    cy.get('.dashboard-row').contains('Errors');
  })
  it('Errors row is collapsed', function() {
    cy.get('.dashboard-row--collapsed').contains('Errors');
  })
  it('Errors row has 3 panels', function(){
    cy.get('.dashboard-row').contains('Errors').find('.dashboard-row__panel_count').contains('(3 panels)');
  })
  it('expands the Errors row', function(){
    cy.get('.dashboard-row__title.pointer').contains('Errors').click();
  })
  it('contains the Errors (by code) panel', function(){
    cy.get('.panel-title').contains('Errors (by code)');
  })
  it('contains the Errors (by operation) panel', function(){
    cy.get('.panel-title').contains('Errors (by operation)');
  })
  it('contains the Errors (by db_type) panel', function(){
    cy.get('.panel-title').contains('Errors (by db_type)');
  })
  it('collapses the Errors row', function(){
    cy.get('.dashboard-row__title.pointer').contains('Errors').click();
    cy.get('.dashboard-row--collapsed').contains('Errors');
  })

  //DURATION ROW
  it('contains the Duration row', function() {
    cy.get('.dashboard-row').contains(/^Duration/);
  })
  it('Duration row is collapsed', function() {
    cy.get('.dashboard-row--collapsed').contains(/^Duration/);
  })
  it('Duration row has 3 panels', function(){
    cy.get('.dashboard-row').contains('Duration').find('.dashboard-row__panel_count').contains('(3 panels)');
  })
  it('expands the Duration row', function(){
    cy.get('.dashboard-row__title.pointer').contains(/^Duration/).click();
  })
  it('contains the Duration (Avg) panel', function () {
    cy.get('.panel-title').contains('Duration (Avg)');
  })
  it('contains the Duration 50th quantile panel', function () {
    cy.get('.panel-title').contains('Duration 50th quantile');
  })
  it('contains the Duration 95th quantile panel', function () {
    cy.get('.panel-title').contains('Duration 95th quantile');
  })
  it('collapses the Duration row', function(){
    cy.get('.dashboard-row__title.pointer').contains(/^Duration/).click();
    cy.get('.dashboard-row--collapsed').contains(/^Duration/);
  })

  //OS ROW
  it('contains the OS row', function() {
    cy.get('.dashboard-row').contains('OS');
  })
  it('OS row is collapsed', function() {
    cy.get('.dashboard-row--collapsed').contains('OS');
  })
  it('OS row has 4 panels', function(){
    cy.get('.dashboard-row').contains('OS').find('.dashboard-row__panel_count').contains('(4 panels)');
  })
  it('expands the OS row', function(){
    cy.get('.dashboard-row__title.pointer').contains('OS').click();
  })
  it('contains the CPU Usage panel', function () {
    cy.get('.panel-title').contains('CPU Usage');
  })
  it('contains the Memory Usage panel', function () {
    cy.get('.panel-title').contains('Memory Usage');
  })
  it('contains the Network Usage panel', function () {
    cy.get('.panel-title').contains('Network Usage');
  })
  it('contains the TCP Retransmissions panel', function () {
    cy.get('.panel-title').contains('TCP Retransmissions');
  })
  it('collapses the OS row', function(){
    cy.get('.dashboard-row__title.pointer').contains('OS').click();
    cy.get('.dashboard-row--collapsed').contains('OS');
  })
})