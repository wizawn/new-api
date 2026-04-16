import React, { useState } from 'react';
import { Button, Typography } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';

const DocsMirrorFrame = ({ src, title }) => {
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [nonce, setNonce] = useState(0);

  return (
    <div className='relative h-screen w-full bg-white'>
      {!loaded && !failed && (
        <div className='absolute inset-0 z-10 flex items-center justify-center bg-white'>
          <Typography.Text className='text-gray-600'>
            Loading documentation...
          </Typography.Text>
        </div>
      )}

      {failed && (
        <div className='absolute inset-0 z-20 flex flex-col items-center justify-center gap-3 bg-white px-4 text-center'>
          <Typography.Title heading={5} className='!mb-0 !text-gray-900'>
            Failed to load documentation
          </Typography.Title>
          <Typography.Text className='text-gray-600'>
            Please retry, or open the page directly.
          </Typography.Text>
          <div className='flex items-center gap-2'>
            <Button
              icon={<IconRefresh />}
              onClick={() => {
                setFailed(false);
                setLoaded(false);
                setNonce((v) => v + 1);
              }}
            >
              Retry
            </Button>
            <Button
              theme='solid'
              type='primary'
              onClick={() => window.open(src, '_blank', 'noopener,noreferrer')}
            >
              Open Directly
            </Button>
          </div>
        </div>
      )}

      <iframe
        title={title}
        src={src}
        key={nonce}
        style={{
          display: 'block',
          width: '100%',
          height: '100vh',
          border: 0,
          backgroundColor: '#ffffff',
        }}
        onLoad={() => {
          setLoaded(true);
          setFailed(false);
        }}
        onError={() => {
          setFailed(true);
          setLoaded(false);
        }}
        allow='clipboard-read; clipboard-write'
        sandbox='allow-scripts allow-same-origin allow-popups allow-forms allow-downloads allow-top-navigation-by-user-activation'
        referrerPolicy='no-referrer'
      />
    </div>
  );
};

export default DocsMirrorFrame;
