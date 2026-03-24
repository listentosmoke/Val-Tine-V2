package com.devicehealth.service;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;

public class AgentService extends Service {

    private static final String TAG = "DeviceHealth";
    private static final String CHANNEL_ID = "device_health_channel";
    private static final int NOTIF_ID = 1;

    private PowerManager.WakeLock wakeLock;
    private static boolean agentLoaded = false;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification notification = buildNotification();
        startForeground(NOTIF_ID, notification);

        acquireWakeLock();
        loadAgent();

        return START_STICKY;
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
        releaseWakeLock();
    }

    @Override
    public void onTaskRemoved(Intent rootIntent) {
        // Restart service if task is removed (swiped away)
        Intent restartIntent = new Intent(this, AgentService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(restartIntent);
        } else {
            startService(restartIntent);
        }
        super.onTaskRemoved(rootIntent);
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                CHANNEL_ID,
                "Device Health",
                NotificationManager.IMPORTANCE_MIN
            );
            channel.setShowBadge(false);
            channel.setSound(null, null);
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) nm.createNotificationChannel(channel);
        }
    }

    private Notification buildNotification() {
        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            builder = new Notification.Builder(this, CHANNEL_ID);
        } else {
            builder = new Notification.Builder(this);
        }
        return builder
            .setContentTitle("Device Health")
            .setContentText("Monitoring device performance")
            .setSmallIcon(android.R.drawable.ic_menu_info_details)
            .setOngoing(true)
            .setPriority(Notification.PRIORITY_MIN)
            .build();
    }

    /**
     * Load the Go agent shared library. The agent starts automatically
     * via Go's init() function which launches a background goroutine.
     * System.loadLibrary uses the OS dynamic linker, which is allowed
     * to load .so files from the native lib directory (bypasses W^X).
     */
    private void loadAgent() {
        if (agentLoaded) {
            Log.i(TAG, "Agent already loaded");
            return;
        }
        try {
            System.loadLibrary("agent");
            agentLoaded = true;
            Log.i(TAG, "Agent shared library loaded — agent started in background");
        } catch (UnsatisfiedLinkError e) {
            Log.e(TAG, "Failed to load agent library: " + e.getMessage(), e);
        }
    }

    private void acquireWakeLock() {
        try {
            PowerManager pm = (PowerManager) getSystemService(POWER_SERVICE);
            if (pm != null) {
                wakeLock = pm.newWakeLock(
                    PowerManager.PARTIAL_WAKE_LOCK,
                    "devicehealth:agent"
                );
                wakeLock.acquire();
            }
        } catch (Exception ignored) {}
    }

    private void releaseWakeLock() {
        try {
            if (wakeLock != null && wakeLock.isHeld()) {
                wakeLock.release();
            }
        } catch (Exception ignored) {}
    }
}
