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
          m('.col', m('.card.card-inverse.card-info.mb-3.text-center', m('.card-block', app.totals.active))),
          m('.col', m('.card.card-inverse.card-primary.mb-3.text-center', m('.card-block', app.totals.queued))),
          m('.col', m('.card.card-inverse.card-success.mb-3.text-center', m('.card-block', app.totals.processed))),
          m('.col', m('.card.card-inverse.card-danger.mb-3.text-center', m('.card-block', app.totals.failed))),
          m('.col', m('.card.card-inverse.card-warning.mb-3.text-center', m('.card-block', app.totals.requeued))),
        ]),

        m('table.table', [
          m('thead.thead-default',
            m('tr', [
              m('th', 'Job ID'),
              m('th', 'Job Tag'),
              m('th', 'Worker'),
              m('th', 'Status'),
            ]),
          ),
          m('tbody',
            map(app.jobs, job => m('tr', { key: job.id }, [
              m('td', job.id),
              m('td', job.tag),
              m('td', job.worker),
              m('td', job.status),
            ])),
          ),
        ]),
      ]),
    ]);
  },
};

export default Dashboard;
