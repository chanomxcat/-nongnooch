import '../config.dart';
import 'api.dart';

/// One CMS item as returned by GET /api/content/public/items (docs/demo-cms-app-contract.md).
class ContentItem {
  ContentItem(this.attrs, this.texts);
  final Map<String, dynamic> attrs;
  final Map<String, dynamic> texts;

  factory ContentItem.fromJson(Map<String, dynamic> j) =>
      ContentItem(Map<String, dynamic>.from(j['attrs'] as Map? ?? {}), Map<String, dynamic>.from(j['texts'] as Map? ?? {}));

  String t(String k) => (texts[k] ?? '').toString();
  String? get imageUrl => attrs['imageUrl'] as String?;
  List<String> get times => ((attrs['times'] as List?) ?? const []).map((e) => e.toString()).toList();
}

class ContentResult {
  ContentResult(this.items, {required this.fromCms});
  final List<ContentItem> items;
  final bool fromCms;
}

/// Loads CMS content and falls back to bundled content when the API fails or returns nothing.
class ContentRepo {
  ContentRepo(this.api);
  final Api api;

  Future<ContentResult> load(String type, String lang) async {
    try {
      final j = await api.get('/api/content/public/items?type=$type&lang=$lang');
      final items = ((j as Map)['items'] as List? ?? const [])
          .map((e) => ContentItem.fromJson(Map<String, dynamic>.from(e as Map)))
          .toList();
      if (items.isNotEmpty) return ContentResult(items, fromCms: true);
    } catch (_) {
      // fall through to bundled content
    }
    return ContentResult(fallbackContent(type, lang), fromCms: false);
  }
}

/// Absolute URL or bundled asset for a CMS imageUrl.
/// `asset:` = bundled image, `/x` = path on the website, otherwise absolute.
String resolveImage(String url) {
  if (url.startsWith('asset:')) return url;
  if (url.startsWith('/')) return '${Config.webBase}$url';
  return url;
}

List<ContentItem> fallbackContent(String type, String lang) {
  final byLang = _fallback[type] ?? const {};
  final list = byLang[lang] ?? byLang['en'] ?? const [];
  return list.map((m) => ContentItem(Map<String, dynamic>.from(m['attrs'] as Map), Map<String, dynamic>.from(m['texts'] as Map))).toList();
}

// Bundled demo content (th + en; other languages fall back to en).
const Map<String, Map<String, List<Map<String, Object>>>> _fallback = {
  'hero_slide': {
    'th': [
      {'attrs': {..._h1}, 'texts': {'title': 'สวนสวยระดับโลก', 'subtitle': 'สวนสวยกว่า 60 โซน', 'ctaLabel': 'ซื้อบัตร'}},
      {'attrs': {..._h2}, 'texts': {'title': 'การแสดงวัฒนธรรมไทย', 'subtitle': 'ชมได้ทุกวัน หลายรอบ', 'ctaLabel': 'ดูรอบการแสดง'}},
      {'attrs': {..._h3}, 'texts': {'title': 'หุบเขาไดโนเสาร์', 'subtitle': 'ไดโนเสาร์ขนาดเท่าของจริง', 'ctaLabel': 'ซื้อบัตร'}},
    ],
    'en': [
      {'attrs': {..._h1}, 'texts': {'title': 'Beautiful Garden', 'subtitle': '60+ themed zones', 'ctaLabel': 'Buy tickets'}},
      {'attrs': {..._h2}, 'texts': {'title': 'Thai Cultural Show', 'subtitle': 'Several rounds every day', 'ctaLabel': 'See show times'}},
      {'attrs': {..._h3}, 'texts': {'title': 'Dinosaur Valley', 'subtitle': 'Life-size dinosaurs', 'ctaLabel': 'Buy tickets'}},
    ],
  },
  'show': {
    'th': [
      {'attrs': {..._s1}, 'texts': {'name': 'การแสดงวัฒนธรรมไทย', 'venue': 'โรงละครใหญ่', 'description': 'รำไทย มวยไทย และการแสดงพื้นบ้าน'}},
      {'attrs': {..._s2}, 'texts': {'name': 'ทัวร์หุบเขาไดโนเสาร์', 'venue': 'หุบเขาไดโนเสาร์', 'description': 'นั่งรถรางชมไดโนเสาร์'}},
    ],
    'en': [
      {'attrs': {..._s1}, 'texts': {'name': 'Thai Cultural Show', 'venue': 'Main theatre', 'description': 'Thai dance, Muay Thai and folk performances'}},
      {'attrs': {..._s2}, 'texts': {'name': 'Dinosaur Valley Tour', 'venue': 'Dinosaur Valley', 'description': 'Tram ride among the dinosaurs'}},
    ],
  },
  'faq': {
    'th': [
      {'attrs': {'order': 1}, 'texts': {'question': 'สวนเปิดกี่โมง', 'answer': 'เปิดทุกวัน 08:00–18:00 น.'}},
      {'attrs': {'order': 2}, 'texts': {'question': 'ใช้บัตรอย่างไร', 'answer': 'แสดง QR ในหน้า "บัตรของฉัน" ที่ประตูทางเข้า ใช้ได้แม้ไม่มีอินเทอร์เน็ต'}},
      {'attrs': {'order': 3}, 'texts': {'question': 'ราคาคนไทยใช้อย่างไร', 'answer': 'ยืนยันเลขบัตรประชาชนในบัญชีก่อนซื้อบัตร'}},
      {'attrs': {'order': 4}, 'texts': {'question': 'พ้อยใช้แลกอะไรได้บ้าง', 'answer': 'เปิดแลกพ้อยเร็ว ๆ นี้ ตอนนี้สะสมได้จากการซื้อบัตร'}},
    ],
    'en': [
      {'attrs': {'order': 1}, 'texts': {'question': 'What are the opening hours?', 'answer': 'Open daily 08:00–18:00.'}},
      {'attrs': {'order': 2}, 'texts': {'question': 'How do I use my ticket?', 'answer': 'Show the QR in "My tickets" at the gate. It works offline.'}},
      {'attrs': {'order': 3}, 'texts': {'question': 'Who gets the Thai price?', 'answer': 'Accounts with a verified Thai ID card number.'}},
      {'attrs': {'order': 4}, 'texts': {'question': 'What can I do with points?', 'answer': 'Redemption opens soon. You earn points with every ticket purchase.'}},
    ],
  },
};
const _h1 = {'order': 1, 'imageUrl': 'asset:assets/images/act-beautiful-garden.webp', 'ctaHref': '/checkout'};
const _h2 = {'order': 2, 'imageUrl': 'asset:assets/images/act-thai-cultural-show.webp', 'ctaHref': '/checkout'};
const _h3 = {'order': 3, 'imageUrl': 'asset:assets/images/act-dinosaur-valley.webp', 'ctaHref': '/checkout'};
const _s1 = {'order': 1, 'times': ['10:30', '13:30', '15:00', '16:30'], 'imageUrl': 'asset:assets/images/act-thai-cultural-show.webp'};
const _s2 = {'order': 2, 'times': ['11:00', '14:00', '15:30'], 'imageUrl': 'asset:assets/images/act-dinosaur-valley.webp'};
