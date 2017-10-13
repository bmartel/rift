import m from 'mithril';
import { map } from 'lodash';
import stats from '../models/job';
import './dashboard.css';

const Dashboard = {
  oninit() {
    this.vm = stats;
  },

  view() {
    return m('.dashboard', [
      m('h1', 'Rift Monitoring'),

      map(this.vm.apps(), (app, name) => [
        m('h2', name),

        m('.row', [
          m('.col', m('.card.card-inverse.card-info.mb-3.text-center', m('.card-block.d-flex.flex-column.justify-content-center', [m('span.text-white', 'Active'), m('span', app.totals.active_jobs)]))),
          m('.col', m('.card.card-inverse.card-primary.mb-3.text-center', m('.card-block.d-flex.flex-column.justify-content-center', [m('span.text-white', 'Queued'), m('span', app.totals.queued_jobs)]))),
          m('.col', m('.card.card-inverse.card-success.mb-3.text-center', m('.card-block.d-flex.flex-column.justify-content-center', [m('span.text-white', 'Processed'), m('span', app.totals.processed_jobs)]))),
          m('.col', m('.card.card-inverse.card-danger.mb-3.text-center', m('.card-block.d-flex.flex-column.justify-content-center', [m('span.text-white', 'Failed'), m('span', app.totals.failed_jobs)]))),
          m('.col', m('.card.card-inverse.card-warning.mb-3.text-center', m('.card-block.d-flex.flex-column.justify-content-center', [m('span.text-white', 'Retried'), m('span', app.totals.requeued_jobs)]))),
        ]),

        m('table.table', [
          m('thead.thead-default',
            m('tr', [
              m('th', 'Job ID'),
              m('th', 'Queue'),
              m('th', 'Job Tag'),
              m('th', 'Status'),
            ]),
          ),
          m('tbody',
            map(app.jobs, job => m('tr', { key: job.id }, [
              m('td', job.id),
              m('td', job.queue_id),
              m('td', job.tag),
              m('td', job.status),
            ])),
          ),
        ]),
      ]),
    ]);
  },
};

export default Dashboard;
