import React from 'react';
import { Typography } from '@douyinfe/semi-ui';
import { CheckCircle2 } from 'lucide-react';

const AuthShell = ({
  logo,
  systemName,
  badge,
  title,
  description,
  highlights,
  children,
}) => {
  return (
    <div className='relative min-h-screen overflow-hidden px-4 pb-10 pt-24 sm:px-6 lg:px-8'>
      <div className='pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_top_left,rgba(59,130,246,0.16),transparent_36%),radial-gradient(circle_at_bottom_right,rgba(34,197,94,0.12),transparent_32%),linear-gradient(180deg,#f5f9ff_0%,#eef5ff_42%,#f8fbff_100%)]' />
      <div className='pointer-events-none absolute left-[-10rem] top-20 -z-10 h-80 w-80 rounded-full bg-blue-300/35 blur-[120px]' />
      <div className='pointer-events-none absolute right-[-12rem] top-52 -z-10 h-96 w-96 rounded-full bg-cyan-300/25 blur-[120px]' />

      <div className='mx-auto grid max-w-7xl gap-8 lg:grid-cols-[1.08fr_minmax(420px,520px)]'>
        <section className='hidden rounded-[36px] border border-blue-100 bg-white/92 px-8 py-10 text-slate-900 shadow-[var(--shadow-floating)] lg:flex lg:flex-col lg:justify-between'>
          <div>
            <div className='inline-flex items-center gap-3 rounded-full border border-blue-100 bg-white px-4 py-2'>
              <img
                src={logo}
                alt={systemName}
                className='h-8 w-8 rounded-full bg-blue-50 p-1 object-contain'
              />
              <Typography.Text className='!text-sm !font-semibold !text-slate-900'>
                {systemName}
              </Typography.Text>
            </div>

            <div className='mt-8 inline-flex rounded-full border border-blue-200 bg-blue-50 px-4 py-2 text-xs font-semibold uppercase tracking-[0.28em] text-blue-700'>
              {badge}
            </div>

            <Typography.Title
              heading={1}
              className='!mb-4 !mt-6 !text-[44px] !leading-[1.05] !text-slate-900'
            >
              {title}
            </Typography.Title>

            <Typography.Paragraph className='!mb-0 !max-w-xl !text-lg !leading-8 !text-slate-600'>
              {description}
            </Typography.Paragraph>
          </div>

          <div className='mt-10 space-y-4'>
            {highlights.map((item) => (
              <div
                key={item}
                className='flex items-start gap-3 rounded-3xl border border-blue-100 bg-blue-50/60 px-4 py-4'
              >
                <div className='mt-0.5 rounded-full bg-blue-100 p-2 text-blue-700'>
                  <CheckCircle2 size={18} />
                </div>
                <Typography.Text className='!text-sm !leading-6 !text-slate-700'>
                  {item}
                </Typography.Text>
              </div>
            ))}
          </div>
        </section>

        <section className='flex min-w-0 items-center justify-center'>{children}</section>
      </div>
    </div>
  );
};

export default AuthShell;
