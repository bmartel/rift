import m from 'mithril';
import './layout.css';

// const active = current => (current === m.route.get() ? 'active' : '');

const Layout = {
  view(vnode) {
    return m('.container', [
      m('header', [
      ]),
      vnode.children,
    ]);
  },
};

export default Layout;
